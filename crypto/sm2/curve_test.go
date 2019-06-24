package sm2

import (
	"math/big"
	"testing"
)

func TestIsOnCurve(t *testing.T) {
	c := Sm2Curve()

	if !c.IsOnCurve(c.Gx, c.Gy) {
		t.Errorf("point (Gx, Gy) is not on Curve")
	}

	if !c.IsOnCurve(c.Gx, new(big.Int).Sub(c.P, c.Gy)) {
		t.Errorf("point (Gx, -Gy) is not on Curve")
	}
}

func TestAddNDouble(t *testing.T) {
	c := Sm2Curve()
	dx, dy := c.Double(c.Gx, c.Gy)
	if !c.IsOnCurve(dx, dy) {
		t.Errorf("double point is not on Curve")
	}

	ax, ay := c.Add(c.Gx, c.Gy, dx, dy)
	if !c.IsOnCurve(ax, ay) {
		t.Errorf("add point is not on Curve")
	}
}

func TestDoubles(t *testing.T) {
	c := Sm2Curve()
	dx, dy := c.Double(c.Gx, c.Gy)
	if !c.IsOnCurve(dx, dy) {
		t.Errorf("double point is not on Curve")
	}

	ax, ay := c.Double(dx, dy)
	if !c.IsOnCurve(ax, ay) {
		t.Errorf("add point is not on Curve")
	}
}

func TestScalarBaseMult(t *testing.T) {
	b := []byte{0x02}
	c := Sm2Curve()

	dx, dy := c.ScalarBaseMult(b)
	if !c.IsOnCurve(dx, dy) {
		t.Errorf("double point is not on Curve")
	}
	d1x, d1y := c.Double(c.Gx, c.Gy)
	if dx.Cmp(d1x) != 0 {
		t.Errorf("ScalarBaseMult result do not equal to double")
	}
	if dy.Cmp(d1y) != 0 {
		t.Errorf("ScalarBaseMult result do not equal to double")
	}
}

func TestScalarBaseMult2(t *testing.T) {
	b := []byte{0x05}
	c := Sm2Curve()

	dx, dy := c.ScalarBaseMult(b)
	if !c.IsOnCurve(dx, dy) {
		t.Errorf("double point is not on Curve")
	}
	d1x, d1y := c.Double(c.Gx, c.Gy)
	d1x, d1y = c.Double(d1x, d1y)
	d1x, d1y = c.Add(d1x, d1y, c.Gx, c.Gy)
	if dx.Cmp(d1x) != 0 {
		t.Errorf("ScalarBaseMult result do not equal to double")
	}
	if dy.Cmp(d1y) != 0 {
		t.Errorf("ScalarBaseMult result do not equal to double")
	}
}
