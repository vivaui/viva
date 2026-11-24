package main

import (
	"bytes"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// An optional provisioning profile enables the Data Protection Keychain. Never
// synthesize restricted entitlements without a profile that authorizes them.
// Ordinary Developer ID/ad-hoc packaging remains unchanged when no profile is
// supplied. CMS decoding and final signing verification use Apple's tools.
func (signing macSigning) prepareProvisioning(run toolRunner, bundleID string) (macSigning, func(), error) {
	noop := func() {}
	if !strings.EqualFold(filepath.Ext(signing.provisionProfile), ".provisionprofile") {
		return signing, noop, errors.New("macOS provisioning profile must be a .provisionprofile file")
	}
	data, err := os.ReadFile(signing.provisionProfile)
	if err != nil {
		return signing, noop, fmt.Errorf("read provisioning profile: %w", err)
	}
	if len(data) == 0 || len(data) > 4*1024*1024 {
		return signing, noop, errors.New("invalid provisioning profile size")
	}
	directory, err := os.MkdirTemp("", "viva-provision-")
	if err != nil {
		return signing, noop, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	success := false
	defer func() {
		if !success {
			cleanup()
		}
	}()
	// Freeze the profile once; validate and embed these same bytes even when a
	// caller changes its source path while the application is being built.
	profilePath := filepath.Join(directory, "embedded.provisionprofile")
	if err = os.WriteFile(profilePath, data, 0o600); err != nil {
		return signing, noop, err
	}
	xmlData, err := verifiedProfilePlist(run, profilePath)
	if err != nil {
		return signing, noop, err
	}
	profile, err := plistDictionary(xmlData)
	if err != nil {
		return signing, noop, fmt.Errorf("invalid provisioning profile: %w", err)
	}
	platforms := plistStrings(profile["Platform"])
	if !containsString(platforms, "OSX") && !containsString(platforms, "macOS") {
		return signing, noop, errors.New("provisioning profile does not target macOS")
	}
	expires := profile["ExpirationDate"]
	expiration, err := time.Parse(time.RFC3339, strings.TrimSpace(expires.Text))
	if expires.XMLName.Local != "date" || err != nil || !expiration.After(time.Now()) {
		return signing, noop, errors.New("provisioning profile has expired or has no valid expiration date")
	}
	grants, err := dictionary(profile["Entitlements"])
	if err != nil {
		return signing, noop, errors.New("provisioning profile has no entitlements dictionary")
	}
	teams := plistStrings(profile["TeamIdentifier"])
	if len(teams) != 1 || teams[0] == "" {
		return signing, noop, errors.New("provisioning profile must identify exactly one development team")
	}
	appGrant := grants["com.apple.application-identifier"]
	if appGrant.XMLName.Local == "" {
		appGrant = grants["application-identifier"]
	}
	pattern := strings.TrimSpace(appGrant.Text)
	prefix, _, ok := strings.Cut(pattern, ".")
	if !ok || prefix == "" || bundleID == "" || strings.ContainsAny(bundleID, "* \t\r\n") {
		return signing, noop, errors.New("profile has no application identifier authorization")
	}
	identifier := prefix + "." + bundleID
	if appGrant.XMLName.Local != "string" || !authorizedString(identifier, pattern) {
		return signing, noop, errors.New("provisioning profile does not authorize the application's bundle identifier")
	}
	if prefixes := plistStrings(profile["ApplicationIdentifierPrefix"]); len(prefixes) != 0 && !containsString(prefixes, prefix) {
		return signing, noop, errors.New("profile application identifier prefix does not match its authorization")
	}
	entitlements := map[string]plistElement{}
	if signing.entitlements != "" {
		existing, err := os.ReadFile(signing.entitlements)
		if err != nil {
			return signing, noop, err
		}
		entitlements, err = plistDictionary(existing)
		if err != nil {
			return signing, noop, fmt.Errorf("invalid existing entitlements: %w", err)
		}
	}
	for _, key := range []string{"com.apple.application-identifier", "application-identifier"} {
		if existing, ok := entitlements[key]; ok && (existing.XMLName.Local != "string" || existing.Text != identifier) {
			return signing, noop, errors.New("existing application identifier entitlement does not match the profile and bundle")
		}
	}
	if existing, ok := entitlements["com.apple.developer.team-identifier"]; ok && (existing.XMLName.Local != "string" || existing.Text != teams[0]) {
		return signing, noop, errors.New("existing team entitlement does not match the provisioning profile")
	}
	groups := plistStrings(entitlements["keychain-access-groups"])
	if existing, ok := entitlements["keychain-access-groups"]; ok && (existing.XMLName.Local != "array" || len(groups) != len(existing.Children)) {
		return signing, noop, errors.New("keychain access groups entitlement must be an array of strings")
	}
	allowedGroups := plistStrings(grants["keychain-access-groups"])
	for _, group := range groups {
		allowed := false
		for _, pattern := range allowedGroups {
			allowed = allowed || authorizedString(group, pattern)
		}
		if !allowed {
			return signing, noop, errors.New("provisioning profile does not authorize a requested keychain access group")
		}
	}
	entitlements["com.apple.application-identifier"] = plistString(identifier)
	entitlements["com.apple.developer.team-identifier"] = plistString(teams[0])
	merged, err := marshalPlist(entitlements)
	if err != nil {
		return signing, noop, err
	}
	entitlementsPath := filepath.Join(directory, "entitlements.plist")
	if err = os.WriteFile(entitlementsPath, merged, 0o600); err != nil {
		return signing, noop, err
	}
	certificates := profile["DeveloperCertificates"]
	if certificates.XMLName.Local != "array" || len(certificates.Children) == 0 {
		return signing, noop, errors.New("provisioning profile has no authorized signing certificates")
	}
	for _, certificate := range certificates.Children {
		if certificate.XMLName.Local != "data" {
			return signing, noop, errors.New("invalid profile signing certificate")
		}
		der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(certificate.Text), ""))
		if err != nil || len(der) == 0 {
			return signing, noop, errors.New("invalid profile signing certificate")
		}
		signing.profileCertificates = append(signing.profileCertificates, der)
	}
	signing.entitlements, signing.profileData, signing.profileTeam, signing.profileDirectory = entitlementsPath, data, teams[0], directory
	success = true
	return signing, cleanup, nil
}

// Verify the CMS content signature first. The actual signer is written to a
// separate file, then validated by Security.framework's Apple provisioning
// policies. Neither signer names nor diagnostic strings from the plist are
// used to decide whether to trust it.
func verifiedProfilePlist(run toolRunner, profilePath string) ([]byte, error) {
	dir := filepath.Dir(profilePath)
	payload, signer := filepath.Join(dir, "payload.plist"), filepath.Join(dir, "signer.pem")
	const openssl = "/usr/bin/openssl"
	if _, err := run(openssl, "smime", "-verify", "-binary", "-inform", "DER", "-in", profilePath, "-noverify", "-signer", signer, "-out", payload); err != nil {
		return nil, fmt.Errorf("verify provisioning profile content signature: %w", err)
	}
	signerData, err := os.ReadFile(signer)
	if err != nil {
		return nil, err
	}
	leaf, rest := pem.Decode(signerData)
	if leaf == nil || leaf.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("expected exactly one provisioning profile signer")
	}
	chain, err := run(openssl, "pkcs7", "-inform", "DER", "-in", profilePath, "-print_certs")
	if err != nil {
		return nil, fmt.Errorf("read provisioning profile certificates: %w", err)
	}
	certArgs := []string{"-c", signer}
	for encoded := []byte(chain); len(encoded) > 0; {
		cert, tail := pem.Decode(encoded)
		if cert == nil {
			break
		}
		encoded = tail
		if cert.Type != "CERTIFICATE" || bytes.Equal(cert.Bytes, leaf.Bytes) {
			continue
		}
		path := filepath.Join(dir, fmt.Sprintf("chain-%d.der", len(certArgs)/2))
		if err := os.WriteFile(path, cert.Bytes, 0o600); err != nil {
			return nil, err
		}
		certArgs = append(certArgs, "-c", path)
	}
	// Apple OSXProvisioningProfileSigning and iPhoneProvisioningProfileSigning
	// each require an Apple anchor and a provisioning-profile signing leaf.
	for _, policy := range []string{"1.2.840.113635.100.1.40", "1.2.840.113635.100.1.56"} {
		_, err = run("/usr/bin/security", append([]string{"verify-cert", "-q", "-p", policy}, certArgs...)...)
		if err == nil {
			return os.ReadFile(payload)
		}
	}
	return nil, fmt.Errorf("verify Apple provisioning profile signer: %w", err)
}

func (signing macSigning) verifyProfileSignature(run toolRunner, bundle string) error {
	metadata, err := run("codesign", "-d", "--verbose=4", bundle)
	if err != nil {
		return err
	}
	if !containsString(strings.Split(metadata, "\n"), "TeamIdentifier="+signing.profileTeam) {
		return errors.New("bundle signing team does not match the provisioning profile")
	}
	prefix := filepath.Join(signing.profileDirectory, "signed-certificate-")
	if _, err = run("codesign", "-d", "--extract-certificates", prefix, bundle); err != nil {
		return err
	}
	leaf, err := os.ReadFile(prefix + "0")
	if err != nil {
		return fmt.Errorf("read bundle signing certificate: %w", err)
	}
	allowed := false
	for _, certificate := range signing.profileCertificates {
		allowed = allowed || bytes.Equal(leaf, certificate)
	}
	if !allowed {
		return errors.New("bundle certificate is not authorized by the provisioning profile")
	}
	granted, err := run("codesign", "-d", "--entitlements", ":-", bundle)
	if err != nil {
		return err
	}
	xmlData, err := embeddedPlist(granted)
	if err != nil {
		return err
	}
	actual, err := plistDictionary(xmlData)
	if err != nil {
		return err
	}
	expectedData, err := os.ReadFile(signing.entitlements)
	if err != nil {
		return err
	}
	expected, err := plistDictionary(expectedData)
	if err != nil {
		return err
	}
	for _, key := range []string{"com.apple.application-identifier", "com.apple.developer.team-identifier", "keychain-access-groups"} {
		a, b := actual[key], expected[key]
		matches := a.XMLName.Local == "string" && b.XMLName.Local == "string" && a.Text == b.Text
		if key == "keychain-access-groups" {
			matches = a.XMLName.Local == "" && b.XMLName.Local == "" ||
				a.XMLName.Local == "array" && b.XMLName.Local == "array" && len(plistStrings(a)) == len(a.Children) && slices.Equal(plistStrings(a), plistStrings(b))
		}
		if !matches {
			return fmt.Errorf("signed entitlement %s does not match the provisioning profile", key)
		}
	}
	return nil
}

func embeddedPlist(output string) ([]byte, error) {
	start := strings.Index(output, "<plist")
	end := strings.Index(output, "</plist>")
	if start < 0 || end < start {
		return nil, errors.New("native tool returned no XML property list")
	}
	return []byte(output[start : end+len("</plist>")]), nil
}

type plistElement struct {
	XMLName  xml.Name
	Text     string         `xml:",chardata"`
	Children []plistElement `xml:",any"`
}

func plistString(value string) plistElement {
	return plistElement{XMLName: xml.Name{Local: "string"}, Text: value}
}
func plistDictionary(data []byte) (map[string]plistElement, error) {
	if len(data) > 4*1024*1024 {
		return nil, errors.New("plist is too large")
	}
	var root plistElement
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if root.XMLName.Local != "plist" || len(root.Children) != 1 {
		return nil, errors.New("expected one plist dictionary")
	}
	return dictionary(root.Children[0])
}
func dictionary(value plistElement) (map[string]plistElement, error) {
	if value.XMLName.Local != "dict" || len(value.Children)%2 != 0 {
		return nil, errors.New("invalid plist dictionary")
	}
	result := make(map[string]plistElement)
	for i := 0; i < len(value.Children); i += 2 {
		key := value.Children[i]
		if key.XMLName.Local != "key" || key.Text == "" {
			return nil, errors.New("invalid plist key")
		}
		if _, exists := result[key.Text]; exists {
			return nil, errors.New("duplicate plist key")
		}
		result[key.Text] = value.Children[i+1]
	}
	return result, nil
}
func plistStrings(value plistElement) []string {
	if value.XMLName.Local != "array" {
		return nil
	}
	var result []string
	for _, item := range value.Children {
		if item.XMLName.Local != "string" {
			return nil
		}
		result = append(result, strings.TrimSpace(item.Text))
	}
	return result
}
func marshalPlist(values map[string]plistElement) ([]byte, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	root := plistElement{XMLName: xml.Name{Local: "dict"}}
	for _, key := range keys {
		root.Children = append(root.Children, plistElement{XMLName: xml.Name{Local: "key"}, Text: key}, values[key])
	}
	data, err := xml.Marshal(root)
	if err != nil {
		return nil, err
	}
	return []byte(xml.Header + `<plist version="1.0">` + string(data) + `</plist>`), nil
}
func authorizedString(value, pattern string) bool {
	if strings.HasSuffix(pattern, ".*") && strings.Count(pattern, "*") == 1 {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*")) && len(value) > len(pattern)-1
	}
	return value == pattern && !strings.Contains(pattern, "*")
}
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
