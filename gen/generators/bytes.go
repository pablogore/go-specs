package generators

// Bytes returns deterministic adversarial byte sequences.
func Bytes() [][]byte {
	long := make([]byte, 10*1024+1)
	for i := range long {
		long[i] = 'b'
	}
	return [][]byte{
		nil,
		{},
		{0},
		{0xff},
		{0, 0, 0},
		[]byte("invalid"),
		[]byte("v"),
		[]byte("v1.2.3"),
		long,
	}
}
