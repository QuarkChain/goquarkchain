package sm2

import (
	"encoding/hex"
	"math/big"
	"testing"
)

type testSm2SignData struct {
	d  string
	x  string
	y  string
	in string
	r  string
	s  string
}

var testSignData = []testSm2SignData{
	{
		d:  "5DD701828C424B84C5D56770ECF7C4FE882E654CAC53C7CC89A66B1709068B9D",
		x:  "FF6712D3A7FC0D1B9E01FF471A87EA87525E47C7775039D19304E554DEFE0913",
		y:  "F632025F692776D4C13470ECA36AC85D560E794E1BCCF53D82C015988E0EB956",
		in: "0102030405060708010203040506070801020304050607080102030405060708",
		r:  "4ebbdb189413df8d82baa7dc6ef69421e8c2520163e6c76fe8c5b3c354c6a0ea",
		s:  "6bc1dfdcab70d85966bdb2273c4173b3256ba0b80c9b49fc7128c6f3eb5eaabe",
	},
}

func TestSign(t *testing.T) {
	for _, data := range testSignData {
		dBytes, _ := hex.DecodeString(data.d)
		priv := PrivKeyFromBytes(dBytes)
		inBytes, _ := hex.DecodeString(data.in)
		r, s, err := Sm2Sign(priv, nil, inBytes)
		if err != nil {
			t.Error(err.Error())
			return
		}

		result := Sm2Verify((*PublicKey)(&priv.PublicKey), nil, inBytes, r, s)
		if !result {
			t.Error("verify failed")
			return
		}
	}
}

func TestVerify(t *testing.T) {
	for _, data := range testSignData {
		pub := new(PublicKey)
		pub.Curve = Sm2Curve()
		xBytes, _ := hex.DecodeString(data.x)
		yBytes, _ := hex.DecodeString(data.y)
		pub.X = new(big.Int).SetBytes(xBytes)
		pub.Y = new(big.Int).SetBytes(yBytes)
		inBytes, _ := hex.DecodeString(data.in)
		if !pub.Curve.IsOnCurve(pub.X, pub.Y) {
			t.Error("not on curve")
		}

		rb, _ := hex.DecodeString(data.r)
		r := new(big.Int).SetBytes(rb)
		sb, _ := hex.DecodeString(data.s)
		s := new(big.Int).SetBytes(sb)
		result := Sm2Verify(pub, nil, inBytes, r, s)
		if !result {
			t.Error("verify failed")
			return
		}
	}
}
