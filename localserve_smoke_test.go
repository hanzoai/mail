//go:build !integration

package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/mail/dkim"
	"github.com/hanzoai/mail/dns"
	"github.com/hanzoai/mail/imapclient"
	"github.com/hanzoai/mail/mlog"
	"github.com/hanzoai/mail/sasl"
	"github.com/hanzoai/mail/smtpclient"
)

// TestLocalserveLoopbackSmokeHelperProcess acts as an isolated localserve child
// process for TestLocalserveLoopbackSmoke so server lifecycle and package-level
// state do not interfere with other tests in package main.
func TestLocalserveLoopbackSmokeHelperProcess(t *testing.T) {
	if os.Getenv("MOX_LOCALSERVE_SMOKE_CHILD") != "1" {
		return
	}
	dir := os.Getenv("MOX_LOCALSERVE_SMOKE_DIR")
	if os.Getenv("MOX_LOCALSERVE_SMOKE_INITONLY") == "1" {
		os.Args = []string{"mox", "localserve", "-initonly", "-ip", "127.0.0.1", "-dir", dir}
	} else {
		os.Args = []string{"mox", "localserve", "-dir", dir}
	}
	main()
	os.Exit(0)
}

func allocateFreeLoopbackPorts(t *testing.T, n int) []int {
	t.Helper()
	listeners := make([]net.Listener, 0, n)
	ports := make([]int, 0, n)
	for range n {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			for _, l := range listeners {
				_ = l.Close()
			}
			t.Fatalf("allocate free loopback port: %v", err)
		}
		listeners = append(listeners, ln)
		ports = append(ports, ln.Addr().(*net.TCPAddr).Port)
	}
	for _, ln := range listeners {
		_ = ln.Close()
	}
	return ports
}

func buildLocalserveDKIMTXTRecord(t *testing.T, pemPath string) string {
	t.Helper()
	pemBytes, err := os.ReadFile(pemPath)
	tcheck(t, err, "read generated DKIM private key")
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatalf("failed to decode DKIM PEM in %s", pemPath)
	}
	privAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	tcheck(t, err, "parse DKIM PKCS8 private key")
	signer, ok := privAny.(interface{ Public() crypto.PublicKey })
	if !ok {
		t.Fatalf("unexpected DKIM private key type %T", privAny)
	}
	rec := dkim.Record{
		Version:   "DKIM1",
		Key:       "ed25519",
		PublicKey: signer.Public(),
	}
	txt, err := rec.Record()
	tcheck(t, err, "format DKIM TXT record")
	return txt
}

// TestLocalserveLoopbackSmoke starts an isolated loopback localserve instance on
// ephemeral ports and verifies HTTP webmail readiness, authenticated SMTP submission,
// localserve queue delivery with DKIM signing, IMAP retrieval, and DKIM verification.
func TestLocalserveLoopbackSmoke(t *testing.T) {
	// Keep path short so the localserve ctl UNIX socket stays within AF_UNIX length limits.
	dir, err := os.MkdirTemp("", "moxls-")
	tcheck(t, err, "create temp dir for localserve smoke test")
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	exe, err := os.Executable()
	tcheck(t, err, "locate test executable")

	initCmd := exec.Command(exe, "-test.run=^TestLocalserveLoopbackSmokeHelperProcess$")
	initCmd.Env = append(os.Environ(),
		"MOX_LOCALSERVE_SMOKE_CHILD=1",
		"MOX_LOCALSERVE_SMOKE_INITONLY=1",
		"MOX_LOCALSERVE_SMOKE_DIR="+dir,
	)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("localserve -initonly failed: %v\n%s", err, string(out))
	}

	ports := allocateFreeLoopbackPorts(t, 8)
	smtpPort, subPort, subsPort, imapPort, imapsPort, httpPort, httpsPort, metricsPort :=
		ports[0], ports[1], ports[2], ports[3], ports[4], ports[5], ports[6], ports[7]

	confPath := filepath.Join(dir, "mox.conf")
	confBytes, err := os.ReadFile(confPath)
	tcheck(t, err, "read generated mox.conf")
	replacer := strings.NewReplacer(
		"\t\t\tPort: 1025", fmt.Sprintf("\t\t\tPort: %d", smtpPort),
		"\t\t\tPort: 1587", fmt.Sprintf("\t\t\tPort: %d", subPort),
		"\t\t\tPort: 1465", fmt.Sprintf("\t\t\tPort: %d", subsPort),
		"\t\t\tPort: 1143", fmt.Sprintf("\t\t\tPort: %d", imapPort),
		"\t\t\tPort: 1993", fmt.Sprintf("\t\t\tPort: %d", imapsPort),
		"\t\t\tPort: 1080", fmt.Sprintf("\t\t\tPort: %d", httpPort),
		"\t\t\tPort: 1443", fmt.Sprintf("\t\t\tPort: %d", httpsPort),
		"\t\t\tPort: 1081", fmt.Sprintf("\t\t\tPort: %d", metricsPort),
	)
	err = os.WriteFile(confPath, []byte(replacer.Replace(string(confBytes))), 0660)
	tcheck(t, err, "rewrite loopback test ports in mox.conf")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var logs bytes.Buffer
	srvCmd := exec.CommandContext(ctx, exe, "-test.run=^TestLocalserveLoopbackSmokeHelperProcess$")
	srvCmd.Env = append(os.Environ(),
		"MOX_LOCALSERVE_SMOKE_CHILD=1",
		"MOX_LOCALSERVE_SMOKE_DIR="+dir,
	)
	srvCmd.Stdout = &logs
	srvCmd.Stderr = &logs
	tcheck(t, srvCmd.Start(), "start localserve child process")
	t.Cleanup(func() {
		if srvCmd.Process != nil {
			_ = srvCmd.Process.Kill()
			_, _ = srvCmd.Process.Wait()
		}
	})

	imapAddr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", imapPort))
	subAddr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", subPort))
	webmailURL := fmt.Sprintf("http://127.0.0.1:%d/webmail/", httpPort)

	deadline := time.Now().Add(15 * time.Second)
	for {
		c1, e1 := net.DialTimeout("tcp", imapAddr, 200*time.Millisecond)
		c2, e2 := net.DialTimeout("tcp", subAddr, 200*time.Millisecond)
		resp, e3 := http.Get(webmailURL)
		statusOK := resp != nil && resp.StatusCode == http.StatusOK
		if resp != nil {
			_ = resp.Body.Close()
		}
		if c1 != nil {
			_ = c1.Close()
		}
		if c2 != nil {
			_ = c2.Close()
		}
		if e1 == nil && e2 == nil && e3 == nil && statusOK {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("localserve listeners not ready within deadline (imap=%v sub=%v webmail=%v):\n%s", e1, e2, e3, logs.String())
		}
		time.Sleep(100 * time.Millisecond)
	}

	imapConn, err := net.DialTimeout("tcp", imapAddr, 5*time.Second)
	tcheck(t, err, "dial localserve imap")
	defer imapConn.Close()

	log := mlog.New("smoke", nil)
	imapOpts := imapclient.Opts{Logger: slog.Default()}
	imapc, err := imapclient.New(imapConn, &imapOpts)
	tcheck(t, err, "create imap client")
	defer func() {
		_, _ = imapc.Logout()
	}()

	_, err = imapc.Login("mox@localhost", "moxmoxmox")
	tcheck(t, err, "imap login")
	_, err = imapc.Select("Inbox")
	tcheck(t, err, "imap select Inbox")

	err = imapc.WriteCommandf("", "idle")
	tcheck(t, err, "imap start idle")
	_, err = imapc.ReadContinuation()
	tcheck(t, err, "imap read idle continuation")

	type idleEvent struct {
		ut  imapclient.Untagged
		err error
	}
	idleCh := make(chan idleEvent, 4)
	go func() {
		for {
			ut, rerr := imapc.ReadUntagged()
			idleCh <- idleEvent{ut: ut, err: rerr}
			if rerr != nil {
				return
			}
			if _, ok := ut.(imapclient.UntaggedExists); ok {
				return
			}
		}
	}()

	subConn, err := net.DialTimeout("tcp", subAddr, 5*time.Second)
	tcheck(t, err, "dial localserve submission")
	defer subConn.Close()

	localhost := dns.Domain{ASCII: "localhost"}
	smtpc, err := smtpclient.New(ctx, log.Logger, subConn, smtpclient.TLSSkip, false, localhost, localhost, smtpclient.Opts{
		Auth: func(_ []string, _ *tls.ConnectionState) (sasl.Client, error) {
			return sasl.NewClientPlain("mox@localhost", "moxmoxmox"), nil
		},
	})
	tcheck(t, err, "smtp submission client")

	const subject = "Hanzo Mail loopback smoke"
	rawMsg := strings.ReplaceAll(fmt.Sprintf(
		"From: <mox@localhost>\r\nTo: <mox@localhost>\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <smoke-%d@localhost>\r\n\r\nLoopback SMTP -> IMAP -> DKIM smoke test body.\r\n",
		subject,
		time.Now().UTC().Format(time.RFC1123Z),
		time.Now().UnixNano(),
	), "\r\n", "\r\n")

	err = smtpc.Deliver(ctx, "mox@localhost", "mox@localhost", int64(len(rawMsg)), strings.NewReader(rawMsg), false, false, false)
	tcheck(t, err, "submit test message over smtp")
	tcheck(t, smtpc.Close(), "close smtp client")

	waitDeadline := time.After(10 * time.Second)
waitExists:
	for {
		select {
		case ev := <-idleCh:
			tcheck(t, ev.err, "read imap idle notification")
			if _, ok := ev.ut.(imapclient.UntaggedExists); ok {
				break waitExists
			}
		case <-waitDeadline:
			t.Fatalf("timed out waiting for IMAP IDLE delivery notification:\n%s", logs.String())
		}
	}

	tcheck(t, imapc.Writelinef("done"), "finish imap idle")
	_, err = imapc.ReadResponse()
	tcheck(t, err, "read imap idle done response")

	tcheck(t, imapc.WriteCommandf("", "fetch 1 body.peek[]"), "send imap fetch")
	fetchResp, err := imapc.ReadResponse()
	tcheck(t, err, "read imap fetch response")

	var fetchedBody string
	for _, ut := range fetchResp.Untagged {
		uf, ok := ut.(imapclient.UntaggedFetch)
		if !ok {
			continue
		}
		for _, attr := range uf.Attrs {
			if fb, ok := attr.(imapclient.FetchBody); ok {
				fetchedBody = fb.Body
				break
			}
		}
	}
	if fetchedBody == "" {
		t.Fatalf("expected non-empty fetched body, got untagged=%#v", fetchResp.Untagged)
	}
	if !strings.Contains(fetchedBody, "Subject: "+subject) {
		t.Fatalf("fetched message missing expected subject %q:\n%s", subject, fetchedBody)
	}
	if !strings.Contains(fetchedBody, "DKIM-Signature:") {
		t.Fatalf("fetched message missing DKIM-Signature header:\n%s", fetchedBody)
	}
	if !strings.Contains(fetchedBody, "dkim=pass") {
		t.Fatalf("fetched message missing dkim=pass Authentication-Results header:\n%s", fetchedBody)
	}

	txtRecord := buildLocalserveDKIMTXTRecord(t, filepath.Join(dir, "dkim.localserve.privatekey.pkcs8.pem"))
	resolver := dns.MockResolver{
		TXT: map[string][]string{
			"localserve._domainkey.localhost.": {txtRecord},
		},
	}
	prevDKIMLocalserve := dkim.Localserve
	dkim.Localserve = true
	t.Cleanup(func() {
		dkim.Localserve = prevDKIMLocalserve
	})

	results, err := dkim.Verify(ctx, slog.Default(), resolver, false, dkim.DefaultPolicy, bytes.NewReader([]byte(fetchedBody)), false)
	tcheck(t, err, "verify dkim signature on fetched message")
	if len(results) != 1 || results[0].Status != dkim.StatusPass {
		t.Fatalf("expected 1 passing DKIM verification result, got %+v (err=%v)", results, err)
	}
}
