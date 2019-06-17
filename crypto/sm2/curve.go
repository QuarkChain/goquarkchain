package sm2

import (
	"crypto/elliptic"
	"fmt"
	"math/big"
	"sync"
)

type sm2Curve struct {
	*elliptic.CurveParams
	A *big.Int // the constant of the curve equation
}

const (
	byteSize = 32
)

var (
	curve     sm2Curve
	halfOrder *big.Int
	q         *big.Int
)

func initSm2Curve() {
	// http://c.gb688.cn/bzgk/gb/showGb?type=online&hcno=728DEA8B8BB32ACFB6EF4BF449BC3077
	var p256Params *elliptic.CurveParams
	p256Params = &elliptic.CurveParams{Name: "SM2-P256"}
	p256Params.P, _ = new(big.Int).SetString("FFFFFFFEFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00000000FFFFFFFFFFFFFFFF", 16)
	p256Params.N, _ = new(big.Int).SetString("FFFFFFFEFFFFFFFFFFFFFFFFFFFFFFFF7203DF6B21C6052B53BBF40939D54123", 16)
	p256Params.B, _ = new(big.Int).SetString("28E9FA9E9D9F5E344D5A9E4BCF6509A7F39789F515AB8F92DDBCBD414D940E93", 16)
	p256Params.Gx, _ = new(big.Int).SetString("32C4AE2C1F1981195F9904466A39C9948FE30BBFF2660BE1715A4589334C74C7", 16)
	p256Params.Gy, _ = new(big.Int).SetString("BC3736A2F4F6779C59BDCEE36B692153D0A9877CC62A474002DF32E52139F0A0", 16)
	p256Params.BitSize = 256

	A, _ := new(big.Int).SetString("FFFFFFFEFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00000000FFFFFFFFFFFFFFFC", 16)

	curve = sm2Curve{p256Params, A}
	q = new(big.Int).Div(new(big.Int).Add(curve.P,
		big.NewInt(1)), big.NewInt(4))
	halfOrder = new(big.Int).Rsh(curve.N, 1)
}

func Sm2Curve() *sm2Curve {
	initonce.Do(initSm2Curve)
	return &curve
}

func (curve *sm2Curve) Params() *elliptic.CurveParams {
	return curve.CurveParams
}

func (curve *sm2Curve) IsOnCurve(x, y *big.Int) bool {
	// y² = x³ + ax + b
	y2 := new(big.Int).Mul(y, y)
	y2.Mod(y2, curve.P)

	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)

	ax := new(big.Int).Mul(x, curve.A)

	x3.Add(x3, ax)
	x3.Add(x3, curve.B)
	x3.Mod(x3, curve.P)

	return x3.Cmp(y2) == 0
}

func (curve *sm2Curve) Add(x1, y1, x2, y2 *big.Int) (*big.Int, *big.Int) {
	z1 := zForAffine(x1, y1)
	z2 := zForAffine(x2, y2)
	return curve.affineFromJacobian(curve.addJacobian(x1, y1, z1, x2, y2, z2))
}

func (curve *sm2Curve) Double(x1, y1 *big.Int) (*big.Int, *big.Int) {
	z1 := zForAffine(x1, y1)
	return curve.affineFromJacobian(curve.doubleJacobian(x1, y1, z1))
}

func (curve *sm2Curve) ScalarMult(Bx, By *big.Int, k []byte) (*big.Int, *big.Int) {
	//Bz := new(big.Int).SetInt64(1)
	Bz := zForAffine(Bx, By)
	x, y, z := new(big.Int), new(big.Int), new(big.Int)

	for _, byte := range k {
		for bitNum := 0; bitNum < 8; bitNum++ {
			x, y, z = curve.doubleJacobian(x, y, z)
			if byte&0x80 == 0x80 {
				x, y, z = curve.addJacobian(x, y, z, Bx, By, Bz)
			}
			byte <<= 1
		}
	}

	return curve.affineFromJacobian(x, y, z)
}

func (curve *sm2Curve) ScalarBaseMult(k []byte) (*big.Int, *big.Int) {
	return curve.ScalarMult(curve.Gx, curve.Gy, k)
}

// decompressPoint decompresses a point on the given curve given the X point and
// the solution to use.
func (curve *sm2Curve) decompressPoint(x *big.Int, ybit bool) (*big.Int, error) {
	// Y = +-sqrt(x^3 + Ax + B)
	ax := new(big.Int).Mul(curve.A, x)
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)
	x3.Add(x3, ax)
	x3.Add(x3, curve.Params().B)
	x3.Mod(x3, curve.Params().P)

	// Now calculate sqrt mod p of x^3 + Ax + B
	// This code used to do a full sqrt based on tonelli/shanks,
	// but this was replaced by the algorithms referenced in
	// https://bitcointalk.org/index.php?topic=162805.msg1712294#msg1712294
	y := new(big.Int).Exp(x3, q, curve.Params().P)

	if ybit != isOdd(y) {
		y.Sub(curve.Params().P, y)
	}

	// Check that y is a square root of x^3 + Ax + B.
	y2 := new(big.Int).Mul(y, y)
	y2.Mod(y2, curve.Params().P)
	if y2.Cmp(x3) != 0 {
		return nil, fmt.Errorf("invalid square root")
	}

	// Sm2Verify that y-coord has expected parity.
	if ybit != isOdd(y) {
		return nil, fmt.Errorf("ybit doesn't match oddness")
	}

	return y, nil
}

// zForAffine returns a Jacobian Z value for the affine point (x, y). If x and
// y are zero, it assumes that they represent the point at infinity because (0,
// 0) is not on the any of the curves handled here.
func zForAffine(x, y *big.Int) *big.Int {
	z := new(big.Int)
	if x.Sign() != 0 || y.Sign() != 0 {
		z.SetInt64(1)
	}
	return z
}

// affineFromJacobian reverses the Jacobian transform. See the comment at the
// top of the file. If the point is ∞ it returns 0, 0.
func (curve *sm2Curve) affineFromJacobian(x, y, z *big.Int) (xOut, yOut *big.Int) {
	if z.Sign() == 0 {
		return new(big.Int), new(big.Int)
	}

	zinv := new(big.Int).ModInverse(z, curve.P)
	zinvsq := new(big.Int).Mul(zinv, zinv)

	xOut = new(big.Int).Mul(x, zinvsq)
	xOut.Mod(xOut, curve.P)
	zinvsq.Mul(zinvsq, zinv)
	yOut = new(big.Int).Mul(y, zinvsq)
	yOut.Mod(yOut, curve.P)
	return
}

// addJacobian takes two points in Jacobian coordinates, (x1, y1, z1) and
// (x2, y2, z2) and returns their sum, also in Jacobian form.
func (curve *sm2Curve) addJacobian(x1, y1, z1, x2, y2, z2 *big.Int) (*big.Int, *big.Int, *big.Int) {
	// See https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian.html#addition-add-2007-bl
	x3, y3, z3 := new(big.Int), new(big.Int), new(big.Int)
	if z1.Sign() == 0 {
		x3.Set(x2)
		y3.Set(y2)
		z3.Set(z2)
		return x3, y3, z3
	}
	if z2.Sign() == 0 {
		x3.Set(x1)
		y3.Set(y1)
		z3.Set(z1)
		return x3, y3, z3
	}

	z1z1 := new(big.Int).Mul(z1, z1)
	z1z1.Mod(z1z1, curve.P)
	z2z2 := new(big.Int).Mul(z2, z2)
	z2z2.Mod(z2z2, curve.P)

	u1 := new(big.Int).Mul(x1, z2z2)
	u1.Mod(u1, curve.P)
	u2 := new(big.Int).Mul(x2, z1z1)
	u2.Mod(u2, curve.P)
	h := new(big.Int).Sub(u2, u1)
	xEqual := h.Sign() == 0
	if h.Sign() == -1 {
		h.Add(h, curve.P)
	}
	i := new(big.Int).Lsh(h, 1)
	i.Mul(i, i)
	i.Mod(i, curve.P)
	j := new(big.Int).Mul(h, i)

	s1 := new(big.Int).Mul(y1, z2)
	s1.Mul(s1, z2z2)
	s1.Mod(s1, curve.P)
	s2 := new(big.Int).Mul(y2, z1)
	s2.Mul(s2, z1z1)
	s2.Mod(s2, curve.P)
	r := new(big.Int).Sub(s2, s1)
	if r.Sign() == -1 {
		r.Add(r, curve.P)
	}
	yEqual := r.Sign() == 0
	if xEqual && yEqual {
		return curve.doubleJacobian(x1, y1, z1)
	}
	r.Lsh(r, 1)
	v := new(big.Int).Mul(u1, i)

	x3.Set(r)
	x3.Mul(x3, x3)
	x3.Sub(x3, j)
	x3.Sub(x3, v)
	x3.Sub(x3, v)
	if x3.Sign() == -1 {
		x3.Add(x3, curve.P)
	}
	x3.Mod(x3, curve.P)

	y3.Set(r)
	v.Sub(v, x3)
	y3.Mul(y3, v)
	s1.Mul(s1, j)
	s1.Lsh(s1, 1)
	y3.Sub(y3, s1)
	y3.Mod(y3, curve.P)

	z3.Add(z1, z2)
	z3.Mul(z3, z3)
	z3.Sub(z3, z1z1)
	z3.Sub(z3, z2z2)
	z3.Mul(z3, h)
	z3.Mod(z3, curve.P)

	return x3, y3, z3
}

// doubleJacobian takes a point in Jacobian coordinates, (x, y, z), and
// returns its double, also in Jacobian form.
func (curve *sm2Curve) doubleJacobian(x, y, z *big.Int) (*big.Int, *big.Int, *big.Int) {
	// See https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian.html#doubling-dbl-2007-bl

	xx := new(big.Int).Mul(x, x)
	xx = xx.Mod(xx, curve.P)
	yy := new(big.Int).Mul(y, y)
	yy = yy.Mod(yy, curve.P)
	yyyy := new(big.Int).Mul(yy, yy)
	yyyy = yyyy.Mod(yyyy, curve.P)
	zz := new(big.Int).Mul(z, z)
	zz = zz.Mod(zz, curve.P)
	s := new(big.Int).Add(x, yy)
	s = s.Mul(s, s)
	s = s.Sub(s, xx)
	if s.Sign() == -1 {
		s = s.Add(s, curve.P)
	}
	s = s.Sub(s, yyyy)
	if s.Sign() == -1 {
		s.Add(s, curve.P)
	}
	s = s.Lsh(s, 1)
	s = s.Mod(s, curve.P)

	azz2 := new(big.Int).Mul(curve.A, zz)
	azz2 = new(big.Int).Mul(azz2, zz)
	azz2 = azz2.Mod(azz2, curve.P)
	m := new(big.Int).Lsh(xx, 1)
	m = m.Add(m, xx)
	m = m.Add(m, azz2)
	m = m.Mod(m, curve.P)

	s2 := new(big.Int).Lsh(s, 1)
	t := new(big.Int).Mul(m, m)
	t = t.Sub(t, s2)
	if t.Sign() == -1 {
		t = t.Add(t, curve.P)
	}
	t = t.Mod(t, curve.P)

	x3 := t

	yyyy8 := new(big.Int).Lsh(yyyy, 3)
	y3 := new(big.Int).Sub(s, t)
	if y3.Sign() == -1 {
		y3 = y3.Add(y3, curve.P)
	}
	y3 = y3.Mul(m, y3)
	y3 = y3.Sub(y3, yyyy8)
	if y3.Sign() == -1 {
		y3 = y3.Add(y3, curve.P)
	}
	y3 = y3.Mod(y3, curve.P)

	z3 := new(big.Int).Add(y, z)
	z3 = z3.Mul(z3, z3)
	z3 = z3.Sub(z3, yy)
	if z3.Sign() == -1 {
		z3 = z3.Add(z3, curve.P)
	}
	z3 = z3.Sub(z3, zz)
	if z3.Sign() == -1 {
		z3 = z3.Add(z3, curve.P)
	}
	z3.Mod(z3, curve.P)

	return x3, y3, z3
}

var initonce sync.Once
