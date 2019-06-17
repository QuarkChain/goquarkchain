package sm2

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"
)
func decodeHex(hexStr string) []byte {
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		panic("invalid hex string in test source: err " + err.Error() +
			", hex: " + hexStr)
	}

	return b
}

func testSignCompact(t *testing.T, tag string, curve *sm2Curve,
	data []byte, isCompressed bool) {
	tmp, _ := GenerateKey(rand.Reader)
	priv := (*PrivateKey)(tmp)

	hashed := []byte("testing")
	sig, err := SignCompact(curve, priv, hashed, isCompressed)
	if err != nil {
		t.Errorf("%s: error signing: %s", tag, err)
		return
	}

	pk, wasCompressed, err := RecoverCompact(curve, sig, hashed)
	if err != nil {
		t.Errorf("%s: error recovering: %s", tag, err)
		return
	}
	if pk.X.Cmp(priv.X) != 0 || pk.Y.Cmp(priv.Y) != 0 {
		t.Errorf("%s: recovered pubkey doesn't match original "+
			"(%v,%v) vs (%v,%v) ", tag, pk.X, pk.Y, priv.X, priv.Y)
		return
	}
	if wasCompressed != isCompressed {
		t.Errorf("%s: recovered pubkey doesn't match compressed state "+
			"(%v vs %v)", tag, isCompressed, wasCompressed)
		return
	}

	// If we change the compressed bit we should get the same key back,
	// but the compressed flag should be reversed.
	if isCompressed {
		sig[0] -= 4
	} else {
		sig[0] += 4
	}

	pk, wasCompressed, err = RecoverCompact(curve, sig, hashed)
	if err != nil {
		t.Errorf("%s: error recovering (2): %s", tag, err)
		return
	}
	if pk.X.Cmp(priv.X) != 0 || pk.Y.Cmp(priv.Y) != 0 {
		t.Errorf("%s: recovered pubkey (2) doesn't match original "+
			"(%v,%v) vs (%v,%v) ", tag, pk.X, pk.Y, priv.X, priv.Y)
		return
	}
	if wasCompressed == isCompressed {
		t.Errorf("%s: recovered pubkey doesn't match reversed "+
			"compressed state (%v vs %v)", tag, isCompressed,
			wasCompressed)
		return
	}
}

func TestSignCompact(t *testing.T) {
	for i := 0; i < 256; i++ {
		name := fmt.Sprintf("test %d", i)
		data := make([]byte, 32)
		_, err := rand.Read(data)
		if err != nil {
			t.Errorf("failed to read random data for %s", name)
			continue
		}
		compressed := i%2 != 0
		testSignCompact(t, name, Sm2Curve(), data, compressed)
	}
}

// recoveryTests assert basic tests for public key recovery from signatures.
// The cases are borrowed from github.com/fjl/btcec-issue.
var recoveryTests = []struct {
	msg string
	sig string
	pub string
	err error
}{
	{
		// Valid curve point recovered.
		msg: "ce0677bb30baa8cf067c88db9811f4333d131bf8bcf12fe7065d211dce971008",
		sig: "01dacf5a0b0a22d01ecbfabe4a9d4377d08f7e371b213ffa9af487c05a3bffc44831e335f58be06c8f2a336e9f051ffd59b7f67e23c9266066a51114f796814cf0",
		pub: "044cffaedfec48f5e9e029fdab356715d9940054274abb0b126ac13fa148f5ad27b5126a5376a029782ea8ecf1fd47b24a3141554d6acc84b0ce160f72c4417f6c",
	},
	{
		// Invalid curve point recovered.
		msg: "00c547e4f7b0f325ad1e56f57e26c745b09a3e503d86e00e5255ff7f715d3d1c",
		sig: "0100b1693892219d736caba55bdb68216e485557ea6b6af75f37096c9aa6a5a75f00b940b1d03b21e36b0e47e79769f095fe2ab855bd91e3a38756b7d75a9c4549",
		err: fmt.Errorf("invalid square root"),
	},
}

func TestRecoverCompact(t *testing.T) {
	for i, test := range recoveryTests {
		msg := decodeHex(test.msg)
		sig := decodeHex(test.sig)

		// Magic DER constant.
		sig[0] += 27

		pub, _, err := RecoverCompact(Sm2Curve(), sig, msg)

		// Verify that returned error matches as expected.
		if !reflect.DeepEqual(test.err, err) {
			t.Errorf("unexpected error returned from pubkey "+
				"recovery #%d: wanted %v, got %v",
				i, test.err, err)
			continue
		}

		// If check succeeded because a proper error was returned, we
		// ignore the returned pubkey.
		if err != nil {
			continue
		}

		// Otherwise, ensure the correct public key was recovered.
		exPub, _ := ParsePubKey(decodeHex(test.pub))
		if !exPub.IsEqual(pub) {
			t.Errorf("unexpected recovered public key #%d: "+
				"want %v, got %v", i, exPub, pub)
		}
	}
}

