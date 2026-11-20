package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// The API checks a domain's DNS records at most once a minute, and answers
// POST /domains/{id}/verify within that minute with the domain as it is. The
// interval is a variable so the plan tests need not wait.
var domainVerifyInterval = time.Minute

// verificationTimeout is how many minutes to wait for a domain to verify.
func verificationTimeout(m DomainResourceModel) int64 {
	if m.VerificationTimeout.IsNull() || m.VerificationTimeout.IsUnknown() {
		return 30
	}
	return m.VerificationTimeout.ValueInt64()
}

var errDomainNotVerified = errors.New("domain is not verified yet")

// waitForDomainVerification asks Cloud to check the domain's DNS records until
// it is verified, verification has failed, or timeoutMinutes have passed, and
// returns the domain as it last was. Not verifying is a warning: the domain
// exists, and an error would taint it, so the next apply would replace it and
// its DNS records with it.
func waitForDomainVerification(ctx context.Context, c *client.Client, id string, timeoutMinutes int64, diags *diag.Diagnostics) *client.DomainData {
	var domain *client.DomainData
	// One check a minute, the first right away.
	err := client.RetryOnConflict(ctx, int(timeoutMinutes)+1, domainVerifyInterval, func() error {
		d, err := c.VerifyDomain(ctx, id)
		if err != nil {
			return err
		}
		domain = d
		if d.Attributes.Verified() {
			return nil
		}
		if a := d.Attributes.ActionRequired; a != nil && *a == "failed" {
			return fmt.Errorf("verification failed")
		}
		return errDomainNotVerified
	}, func(err error) bool { return errors.Is(err, errDomainNotVerified) })
	if err == nil {
		return domain
	}
	detail := fmt.Sprintf("Domain %s was not verified within %d minutes", id, timeoutMinutes)
	if !errors.Is(err, errDomainNotVerified) {
		detail = fmt.Sprintf("Waiting for domain %s to verify stopped: %s", id, err)
	}
	diags.AddWarning("Domain not verified", detail+". Create the records in dns_records at your DNS provider; "+
		"Laravel Cloud keeps checking them. The statuses in state are the last ones Laravel Cloud reported.")
	return domain
}
