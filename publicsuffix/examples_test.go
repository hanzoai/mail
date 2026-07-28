package publicsuffix_test

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hanzoai/mail/dns"
	"github.com/hanzoai/mail/publicsuffix"
)

func ExampleLookup() {
	// Lookup the organizational domain for sub.example.org.
	orgDom := publicsuffix.Lookup(context.Background(), slog.Default(), dns.Domain{ASCII: "sub.example.org"})
	fmt.Println(orgDom)
	// Output: example.org
}
