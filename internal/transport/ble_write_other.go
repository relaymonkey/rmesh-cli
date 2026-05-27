//go:build !darwin

package transport

func (t *bleTransport) writeToRadio(buf []byte) error {
	_, err := t.toRadio.WriteWithoutResponse(buf)
	return err
}
