// Package term holds end-to-end tests that run the real meno binary in a
// pseudo-terminal and read what it draws through a terminal emulator. They
// need the build tag termtest (make term) and talk only to a fake provider.
package term
