package sm2

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	priv, err := GenerateKey(rand.Reader)
	if err != nil {
		t.Error(err.Error())
		return
	}

	curve := Sm2Curve()
	if !curve.IsOnCurve(priv.X, priv.Y) {
		t.Error("x,y is not on Curve")
		return
	}
}

func TestPrivKeys(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
	}{
		{
			name: "check curve",
			key: []byte{
				0xea, 0xf0, 0x2c, 0xa3, 0x48, 0xc5, 0x24, 0xe6,
				0x39, 0x26, 0x55, 0xba, 0x4d, 0x29, 0x60, 0x3c,
				0xd1, 0xa7, 0x34, 0x7d, 0x9d, 0x65, 0xcf, 0xe9,
				0x3c, 0xe1, 0xeb, 0xff, 0xdc, 0xa2, 0x26, 0x94,
			},
		},
	}

	for _, test := range tests {
		priv := PrivKeyFromBytes(test.key)
		pub := (*PublicKey)(&priv.PublicKey)
		if !curve.IsOnCurve(pub.X, pub.Y) {
			t.Errorf("%s privkey: %v", test.name, "public key is not on curve")
			continue
		}

		_, err := ParsePubKey(pub.SerializeUncompressed())
		if err != nil {
			t.Errorf("%s privkey: %v", test.name, err)
			continue
		}

		hash := []byte{0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9}
		r, s, err := priv.Sign(hash)
		if err != nil {
			t.Errorf("%s could not sign: %v", test.name, err)
			continue
		}

		if !ecdsa.Verify((*ecdsa.PublicKey)(pub), hash, r, s) {
			t.Errorf("%s could not verify: %v", test.name, err)
			continue
		}

		keyBytes := make([]byte, 0)
		if !bytes.Equal(paddedAppend(32, keyBytes, priv.D.Bytes()), test.key) {
			t.Errorf("%s unexpected serialized bytes - got: %x, "+
				"want: %x", test.name, keyBytes, test.key)
		}
	}
}
