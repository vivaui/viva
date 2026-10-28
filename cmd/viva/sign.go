package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// runSign signs a program that was built somewhere else. A release that verifies
// the bytes before packaging them cannot let package --program sign them: a
// supplied program is used as it is, so the signature is applied here, before
// the check that reads the final bytes.
func runSign(arguments []string, output, errorOutput io.Writer) error {
	return signProgram(arguments, os.Getenv, runSigningTool, output, errorOutput)
}

func signProgram(arguments []string, environment func(string) string, run toolRunner, output, errorOutput io.Writer) error {
	c, _ := findCommand("sign")
	flags := flag.NewFlagSet("sign", flag.ContinueOnError)
	var target, program, signCommand string
	var skip bool
	flags.StringVar(&target, "target", "", "target platform to sign for (windows)")
	flags.StringVar(&program, "program", "", "program to sign, in place")
	flags.StringVar(&signCommand, "sign-command", "", "custom sign command, naming the file as {file}")
	flags.BoolVar(&skip, "no-sign", false, "leave the program unsigned")
	positional, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if len(positional) != 0 || target == "" || program == "" {
		return errors.New("sign needs --target and --program; run 'viva help sign'")
	}
	if target != "windows" {
		return fmt.Errorf("sign: only windows programs are signed by viva; not %s", target)
	}
	info, err := os.Stat(program)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("sign: %s is not a program", displayPath(program))
	}
	signing, err := resolveWindowsSigning(signingOptions{skip: skip, signCommand: signCommand}, environment)
	if err != nil {
		return err
	}
	if signing.cleanup != nil {
		defer signing.cleanup()
	}
	if !signing.signs() {
		fmt.Fprintln(output, "  signing  none: no Windows signing options are set (see viva help sign)")
		return nil
	}
	if err := signing.signFile(run, program); err != nil {
		return err
	}
	fmt.Fprintf(output, "  signed   %s\n", displayPath(program))
	return nil
}
