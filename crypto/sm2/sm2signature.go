package sm2

import (
	"crypto/rand"
	"encoding/binary"
	"github.com/ZZMarquis/gm/util"
	"hash"
	"io"
	"math/big"

	"github.com/QuarkChain/goquarkchain/crypto/sm3"
)

var (
	intZero              = new(big.Int).SetInt64(0)
	sm2H                 = new(big.Int).SetInt64(1)
	sm2SignDefaultUserId = []byte{
		0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38,
		0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38}
)

func Sm2Sign(priv *PrivateKey, userId []byte, in []byte) (r, s *big.Int, err error) {
	hash := sm3.NewSM3Hash()
	curve := (priv.Curve).(*sm2Curve)
	pubX, pubY := priv.Curve.ScalarBaseMult(priv.D.Bytes())
	if userId == nil {
		userId = sm2SignDefaultUserId
	}
	e := caculateE(hash, curve, pubX, pubY, userId, in)

	intZero := new(big.Int).SetInt64(0)
	intOne := new(big.Int).SetInt64(1)
	for {
		var k *big.Int
		var err error
		for {
			k, err = nextK(rand.Reader, curve.N)
			if err != nil {
				return nil, nil, err
			}
			px, _ := priv.Curve.ScalarBaseMult(k.Bytes())
			r = util.Add(e, px)
			r = util.Mod(r, curve.N)

			rk := new(big.Int).Set(r)
			rk = rk.Add(rk, k)
			if r.Cmp(intZero) != 0 && rk.Cmp(curve.N) != 0 {
				break
			}
		}

		dPlus1ModN := util.Add(priv.D, intOne)
		dPlus1ModN = util.ModInverse(dPlus1ModN, curve.N)
		s = util.Mul(r, priv.D)
		s = util.Sub(k, s)
		s = util.Mod(s, curve.N)
		s = util.Mul(dPlus1ModN, s)
		s = util.Mod(s, curve.N)

		if s.Cmp(intZero) != 0 {
			break
		}
	}

	return r, s, nil
}

func Sm2Verify(pub *PublicKey, userId []byte, src []byte, r, s *big.Int) bool {
	intOne := new(big.Int).SetInt64(1)
	curve := (pub.Curve).(*sm2Curve)
	if r.Cmp(intOne) == -1 || r.Cmp(curve.N) >= 0 {
		return false
	}
	if s.Cmp(intOne) == -1 || s.Cmp(curve.N) >= 0 {
		return false
	}

	digest := sm3.NewSM3Hash()
	if userId == nil {
		userId = sm2SignDefaultUserId
	}
	e := caculateE(digest, curve, pub.X, pub.Y, userId, src)

	t := util.Add(r, s)
	t = util.Mod(t, curve.N)
	if t.Cmp(intZero) == 0 {
		return false
	}

	sgx, sgy := curve.ScalarBaseMult(s.Bytes())
	tpx, tpy := curve.ScalarMult(pub.X, pub.Y, t.Bytes())
	x, y := curve.Add(sgx, sgy, tpx, tpy)
	if util.IsEcPointInfinity(x, y) {
		return false
	}

	expectedR := util.Add(e, x)
	expectedR = util.Mod(expectedR, curve.N)
	return expectedR.Cmp(r) == 0
}

func nextK(rnd io.Reader, max *big.Int) (*big.Int, error) {
	intOne := new(big.Int).SetInt64(1)
	var k *big.Int
	var err error
	for {
		k, err = rand.Int(rnd, max)
		if err != nil {
			return nil, err
		}
		if k.Cmp(intOne) >= 0 {
			return k, err
		}
	}
}

func getZ(digest hash.Hash, curve *sm2Curve, pubX *big.Int, pubY *big.Int, userId []byte) []byte {
	digest.Reset()

	userIdLen := uint16(len(userId) * 8)
	var userIdLenBytes [2]byte
	binary.BigEndian.PutUint16(userIdLenBytes[:], userIdLen)
	digest.Write(userIdLenBytes[:])
	if userId != nil && len(userId) > 0 {
		digest.Write(userId)
	}

	digest.Write(curve.A.Bytes())
	digest.Write(curve.B.Bytes())
	digest.Write(curve.Gx.Bytes())
	digest.Write(curve.Gy.Bytes())
	digest.Write(pubX.Bytes())
	digest.Write(pubY.Bytes())
	return digest.Sum(nil)
}

func caculateE(digest hash.Hash, curve *sm2Curve, pubX *big.Int, pubY *big.Int, userId []byte, src []byte) *big.Int {
	z := getZ(digest, curve, pubX, pubY, userId)

	digest.Reset()
	digest.Write(z)
	digest.Write(src)
	eHash := digest.Sum(nil)
	return new(big.Int).SetBytes(eHash)
}
