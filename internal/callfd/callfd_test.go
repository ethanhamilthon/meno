package callfd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"testing"
)

// echo writes back what it reads until the peer closes.
func echo(c net.Conn) {
	defer c.Close()
	buf := make([]byte, 64)
	for {
		n, err := c.Read(buf)
		if n > 0 {
			if _, werr := c.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func TestConcurrentConnectionsAreIndependent(t *testing.T) {
	ch, err := Open(echo)
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	f := ch.Files()
	inherited.ctl, inherited.lock = f[0], f[1]
	inherited.once.Do(func() {})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, err := connect(int(f[0].Fd()), int(f[1].Fd()))
			if err != nil {
				t.Error(err)
				return
			}
			defer c.Close()
			msg := []byte{byte('a' + i), byte('a' + i)}
			if _, err = c.Write(msg); err != nil {
				t.Error(err)
				return
			}
			got := make([]byte, 2)
			if _, err = io.ReadFull(c, got); err != nil || string(got) != string(msg) {
				t.Errorf("conn %d: got %q err %v", i, got, err)
			}
		}(i)
	}
	wg.Wait()
}

// TestMain lets the test binary act as the "meno" command of an agent call.
func TestMain(m *testing.M) {
	if os.Getenv("CALLFD_HELPER") == "1" {
		c, err := Dial()(context.Background())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_, _ = c.Write([]byte("ping"))
		buf := make([]byte, 4)
		if _, err = io.ReadFull(c, buf); err != nil || string(buf) != "ping" {
			fmt.Fprintln(os.Stderr, "bad echo", err)
			os.Exit(1)
		}
		fmt.Print("ok")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// Several processes of one call share the descriptors and still get private connections.
func TestChildProcessesShareTheChannel(t *testing.T) {
	ch, err := Open(echo)
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	cmds := make([]*exec.Cmd, 5)
	outs := make([]*bytes.Buffer, 5)
	errs := make([]*bytes.Buffer, 5)
	for i := range cmds {
		cmds[i] = exec.Command(os.Args[0])
		cmds[i].Env = append(os.Environ(), "CALLFD_HELPER=1")
		cmds[i].ExtraFiles = ch.Files()
		outs[i] = &bytes.Buffer{}
		cmds[i].Stdout = outs[i] // stderr may carry coverage notes of the helper
		errs[i] = &bytes.Buffer{}
		cmds[i].Stderr = errs[i]
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for i, c := range cmds {
		if err := c.Wait(); err != nil || outs[i].String() != "ok" {
			t.Errorf("child %d: %v %q stderr %q", i, err, outs[i], errs[i])
		}
	}
}

func TestWithoutDescriptorsDialFails(t *testing.T) {
	if _, err := connect(900, 901); err == nil {
		t.Fatal("dial without inherited descriptors must fail")
	}
}
