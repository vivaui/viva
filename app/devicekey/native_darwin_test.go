//go:build darwin && !ios && cgo

package devicekey

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestKeychainEnrollmentRollsBackOnlyItsOwnSuccessfulAdd(t *testing.T) {
	for _, tc := range []struct {
		name      string
		addStatus int64
		verifyErr error
		want      []int
	}{
		{"item already exists", -25299, nil, []int{keychainAdd}},
		{"user canceled verification", 0, context.Canceled, []int{keychainAdd, keychainRead, keychainDelete}},
		{"verified", 0, nil, []int{keychainAdd, keychainRead}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := []byte("test application key")
			var actions []int
			_, err := enrollInKeychain(context.Background(), Options{}, key, func(ctx context.Context, _ Options, _, _ []byte, action int) (int64, []byte, error) {
				actions = append(actions, action)
				switch action {
				case keychainAdd:
					return tc.addStatus, nil, nil
				case keychainRead:
					return 0, slices.Clone(key), tc.verifyErr
				default:
					return 0, nil, ctx.Err()
				}
			})
			if !slices.Equal(actions, tc.want) || (err == nil) != (tc.addStatus == 0 && tc.verifyErr == nil) {
				t.Fatalf("actions %v, error %v", actions, err)
			}
			if tc.verifyErr != nil && !errors.Is(err, tc.verifyErr) {
				t.Fatalf("verification error lost: %v", err)
			}
		})
	}
}
