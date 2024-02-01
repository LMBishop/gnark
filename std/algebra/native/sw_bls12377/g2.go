/*
Copyright © 2020 ConsenSys

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sw_bls12377

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	bls12377 "github.com/consensys/gnark-crypto/ecc/bls12-377"

	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/algopts"
	"github.com/consensys/gnark/std/algebra/native/fields_bls12377"
)

type g2AffP struct {
	X, Y fields_bls12377.E2
}

// G2Affine point in affine coords
type G2Affine struct {
	P     g2AffP
	Lines *lineEvaluations
}

// Neg outputs -p
func (p *g2AffP) Neg(api frontend.API, p1 g2AffP) *g2AffP {
	p.Y.Neg(api, p1.Y)
	p.X = p1.X
	return p
}

// AddAssign add p1 to p and return p
func (p *g2AffP) AddAssign(api frontend.API, p1 g2AffP) *g2AffP {

	var n, d, l, xr, yr fields_bls12377.E2

	// compute lambda = (p1.y-p.y)/(p1.x-p.x)
	n.Sub(api, p1.Y, p.Y)
	d.Sub(api, p1.X, p.X)
	l.DivUnchecked(api, n, d)

	// xr =lambda**2-p1.x-p.x
	xr.Square(api, l).
		Sub(api, xr, p1.X).
		Sub(api, xr, p.X)

	// yr = lambda(p.x - xr)-p.y
	yr.Sub(api, p.X, xr).
		Mul(api, l, yr).
		Sub(api, yr, p.Y)

	p.X = xr
	p.Y = yr

	return p
}

func (p *g2AffP) AddUnified(api frontend.API, q g2AffP) *g2AffP {
	// selector1 = 1 when p is (0,0) and 0 otherwise
	selector1 := api.And(p.X.IsZero(api), p.Y.IsZero(api))
	// selector2 = 1 when q is (0,0) and 0 otherwise
	selector2 := api.And(q.X.IsZero(api), q.Y.IsZero(api))

	// λ = ((p.x+q.x)² - p.x*q.x + a)/(p.y + q.y)
	var pxqx, pxplusqx, num, denum, λ fields_bls12377.E2
	pxqx.Mul(api, p.X, q.X)
	pxplusqx.Add(api, p.X, q.X)
	num.Mul(api, pxplusqx, pxplusqx)
	num.Sub(api, num, pxqx)
	denum.Add(api, p.Y, q.Y)
	// if p.y + q.y = 0, assign dummy 1 to denum and continue
	selector3 := denum.IsZero(api)
	one := fields_bls12377.E2{A0: 1, A1: 0}
	denum.Select(api, selector3, one, denum)
	λ.DivUnchecked(api, num, denum)

	// x = λ^2 - p.x - q.x
	var xr, yr fields_bls12377.E2
	xr.Square(api, λ)
	xr.Sub(api, xr, pxplusqx)

	// y = λ(p.x - xr) - p.y
	yr.Sub(api, p.X, xr)
	yr.Mul(api, yr, λ)
	yr.Sub(api, yr, p.Y)
	result := g2AffP{
		X: xr,
		Y: yr,
	}

	// if p=(0,0) return q
	result.Select(api, selector1, q, result)
	// if q=(0,0) return p
	result.Select(api, selector2, *p, result)
	// if p.y + q.y = 0, return (0, 0)
	zero := fields_bls12377.E2{A0: 0, A1: 0}
	result.Select(api, selector3, g2AffP{X: zero, Y: zero}, result)

	p.X = result.X
	p.Y = result.Y

	return p
}

// Select sets p1 if b=1, p2 if b=0, and returns it. b must be boolean constrained
func (p *g2AffP) Select(api frontend.API, b frontend.Variable, p1, p2 g2AffP) *g2AffP {

	p.X.Select(api, b, p1.X, p2.X)
	p.Y.Select(api, b, p1.Y, p2.Y)

	return p
}

// Double compute 2*p1, assign the result to p and return it
// Only for curve with j invariant 0 (a=0).
func (p *g2AffP) Double(api frontend.API, p1 g2AffP) *g2AffP {

	var n, d, l, xr, yr fields_bls12377.E2

	// lambda = 3*p1.x**2/2*p.y
	n.Square(api, p1.X).MulByFp(api, n, 3)
	d.MulByFp(api, p1.Y, 2)
	l.DivUnchecked(api, n, d)

	// xr = lambda**2-2*p1.x
	xr.Square(api, l).
		Sub(api, xr, p1.X).
		Sub(api, xr, p1.X)

	// yr = lambda*(p.x-xr)-p.y
	yr.Sub(api, p1.X, xr).
		Mul(api, l, yr).
		Sub(api, yr, p1.Y)

	p.X = xr
	p.Y = yr

	return p

}

func (P *g2AffP) doubleN(api frontend.API, Q *g2AffP, n int) *g2AffP {
	pn := Q
	for s := 0; s < n; s++ {
		pn.Double(api, *pn)
	}
	return pn
}

func (P *g2AffP) scalarMulBySeed(api frontend.API, Q *g2AffP) *g2AffP {
	var z, t0, t1 g2AffP
	z.Double(api, *Q)
	z.AddAssign(api, *Q)
	z.DoubleAndAdd(api, &z, Q)
	t0.Double(api, z)
	t0.Double(api, t0)
	z.AddAssign(api, t0)
	t1.Double(api, z)
	t1.AddAssign(api, z)
	t0.AddAssign(api, t1)
	t0.doubleN(api, &t0, 9)
	z.DoubleAndAdd(api, &t0, &z)
	z.doubleN(api, &z, 45)
	P.DoubleAndAdd(api, &z, Q)

	return P
}

// ScalarMul sets P = [s] Q and returns P.
//
// The method chooses an implementation based on scalar s. If it is constant,
// then the compiled circuit depends on s. If it is variable type, then
// the circuit is independent of the inputs.
func (P *g2AffP) ScalarMul(api frontend.API, Q g2AffP, s interface{}, opts ...algopts.AlgebraOption) *g2AffP {
	if n, ok := api.Compiler().ConstantValue(s); ok {
		return P.constScalarMul(api, Q, n, opts...)
	} else {
		return P.varScalarMul(api, Q, s, opts...)
	}
}

var DecomposeScalarG2 = func(scalarField *big.Int, inputs []*big.Int, res []*big.Int) error {
	cc := getInnerCurveConfig(scalarField)
	sp := ecc.SplitScalar(inputs[0], cc.glvBasis)
	res[0].Set(&(sp[0]))
	res[1].Set(&(sp[1]))
	one := big.NewInt(1)
	// add (lambda+1, lambda) until scalar compostion is over Fr to ensure that
	// the high bits are set in decomposition.
	for res[0].Cmp(cc.lambda) < 1 && res[1].Cmp(cc.lambda) < 1 {
		res[0].Add(res[0], cc.lambda)
		res[0].Add(res[0], one)
		res[1].Add(res[1], cc.lambda)
	}
	// figure out how many times we have overflowed
	res[2].Mul(res[1], cc.lambda).Add(res[2], res[0])
	res[2].Sub(res[2], inputs[0])
	res[2].Div(res[2], cc.fr)

	return nil
}

func init() {
	solver.RegisterHint(DecomposeScalarG2)
}

// varScalarMul sets P = [s] Q and returns P.
func (P *g2AffP) varScalarMul(api frontend.API, Q g2AffP, s frontend.Variable, opts ...algopts.AlgebraOption) *g2AffP {
	cfg, err := algopts.NewConfig(opts...)
	if err != nil {
		panic(err)
	}
	// This method computes [s] Q. We use several methods to reduce the number
	// of added constraints - first, instead of classical double-and-add, we use
	// the optimized version from https://github.com/zcash/zcash/issues/3924
	// which allows to omit computation of several intermediate values.
	// Secondly, we use the GLV scalar multiplication to reduce the number
	// iterations in the main loop. There is a small difference though - as
	// two-bit select takes three constraints, then it takes as many constraints
	// to compute ± Q ± Φ(Q) every iteration instead of selecting the value
	// from a precomputed table. However, precomputing the table adds 12
	// additional constraints and thus table-version is more expensive than
	// addition-version.
	var selector frontend.Variable
	if cfg.CompleteArithmetic {
		// if Q=(0,0) we assign a dummy (1,1) to Q and continue
		selector = api.And(Q.X.IsZero(api), Q.Y.IsZero(api))
		one := fields_bls12377.E2{A0: 1, A1: 0}
		Q.Select(api, selector, g2AffP{X: one, Y: one}, Q)
	}
	// The context we are working is based on the `outer` curve. However, the
	// points and the operations on the points are performed on the `inner`
	// curve of the outer curve. We require some parameters from the inner
	// curve.
	cc := getInnerCurveConfig(api.Compiler().Field())

	// the hints allow to decompose the scalar s into s1 and s2 such that
	//     s1 + λ * s2 == s mod r,
	// where λ is third root of one in 𝔽_r.
	sd, err := api.Compiler().NewHint(DecomposeScalarG2, 3, s)
	if err != nil {
		// err is non-nil only for invalid number of inputs
		panic(err)
	}
	s1, s2 := sd[0], sd[1]

	// when we split scalar, then s1, s2 < lambda by default. However, to have
	// the high 1-2 bits of s1, s2 set, the hint functions compute the
	// decomposition for
	//     s + k*r (for some k)
	// instead and omits the last reduction. Thus, to constrain s1 and s2, we
	// have to assert that
	//     s1 + λ * s2 == s + k*r
	api.AssertIsEqual(api.Add(s1, api.Mul(s2, cc.lambda)), api.Add(s, api.Mul(cc.fr, sd[2])))

	// As the decomposed scalars are not fully reduced, then in addition of
	// having the high bit set, an overflow bit may also be set. Thus, the total
	// number of bits may be one more than the bitlength of λ.
	nbits := cc.lambda.BitLen() + 1

	s1bits := api.ToBinary(s1, nbits)
	s2bits := api.ToBinary(s2, nbits)

	var Acc /*accumulator*/, B, B2 /*tmp vars*/ g2AffP
	// precompute -Q, -Φ(Q), Φ(Q)
	var tableQ, tablePhiQ [2]g2AffP
	tableQ[1] = Q
	tableQ[0].Neg(api, Q)
	cc.phi2(api, &tablePhiQ[1], &Q)
	tablePhiQ[0].Neg(api, tablePhiQ[1])

	// We now initialize the accumulator. Due to the way the scalar is
	// decomposed, either the high bits of s1 or s2 are set and we can use the
	// incomplete addition laws.

	//     Acc = Q + Φ(Q)
	Acc = tableQ[1]
	Acc.AddAssign(api, tablePhiQ[1])

	// However, we can not directly add step value conditionally as we may get
	// to incomplete path of the addition formula. We either add or subtract
	// step value from [2] Acc (instead of conditionally adding step value to
	// Acc):
	//     Acc = [2] (Q + Φ(Q)) ± Q ± Φ(Q)
	// only y coordinate differs for negation, select on that instead.
	B.X = tableQ[0].X
	B.Y.Select(api, s1bits[nbits-1], tableQ[1].Y, tableQ[0].Y)
	Acc.DoubleAndAdd(api, &Acc, &B)
	B.X = tablePhiQ[0].X
	B.Y.Select(api, s2bits[nbits-1], tablePhiQ[1].Y, tablePhiQ[0].Y)
	Acc.AddAssign(api, B)

	// second bit
	B.X = tableQ[0].X
	B.Y.Select(api, s1bits[nbits-2], tableQ[1].Y, tableQ[0].Y)
	Acc.DoubleAndAdd(api, &Acc, &B)
	B.X = tablePhiQ[0].X
	B.Y.Select(api, s2bits[nbits-2], tablePhiQ[1].Y, tablePhiQ[0].Y)
	Acc.AddAssign(api, B)

	B2.X = tablePhiQ[0].X
	for i := nbits - 3; i > 0; i-- {
		B.X = Q.X
		B.Y.Select(api, s1bits[i], tableQ[1].Y, tableQ[0].Y)
		B2.Y.Select(api, s2bits[i], tablePhiQ[1].Y, tablePhiQ[0].Y)
		B.AddAssign(api, B2)
		Acc.DoubleAndAdd(api, &Acc, &B)
	}

	// i = 0
	// When cfg.CompleteArithmetic is set, we use AddUnified instead of Add. This means
	// when s=0 then Acc=(0,0) because AddUnified(Q, -Q) = (0,0).
	if cfg.CompleteArithmetic {
		tableQ[0].AddUnified(api, Acc)
		Acc.Select(api, s1bits[0], Acc, tableQ[0])
		tablePhiQ[0].AddUnified(api, Acc)
		Acc.Select(api, s2bits[0], Acc, tablePhiQ[0])
		zero := fields_bls12377.E2{A0: 0, A1: 0}
		Acc.Select(api, selector, g2AffP{X: zero, Y: zero}, Acc)
	} else {
		tableQ[0].AddAssign(api, Acc)
		Acc.Select(api, s1bits[0], Acc, tableQ[0])
		tablePhiQ[0].AddAssign(api, Acc)
		Acc.Select(api, s2bits[0], Acc, tablePhiQ[0])
	}

	P.X = Acc.X
	P.Y = Acc.Y

	return P
}

// constScalarMul sets P = [s] Q and returns P.
func (P *g2AffP) constScalarMul(api frontend.API, Q g2AffP, s *big.Int, opts ...algopts.AlgebraOption) *g2AffP {
	cfg, err := algopts.NewConfig(opts...)
	if err != nil {
		panic(err)
	}
	if s.BitLen() == 0 {
		zero := fields_bls12377.E2{A0: 0, A1: 0}
		P.X = zero
		P.Y = zero
		return P
	}
	// see the comments in varScalarMul. However, two-bit lookup is cheaper if
	// bits are constant and here it makes sense to use the table in the main
	// loop.
	var Acc, negQ, negPhiQ, phiQ g2AffP
	cc := getInnerCurveConfig(api.Compiler().Field())
	s.Mod(s, cc.fr)
	cc.phi2(api, &phiQ, &Q)

	k := ecc.SplitScalar(s, cc.glvBasis)
	if k[0].Sign() == -1 {
		k[0].Neg(&k[0])
		Q.Neg(api, Q)
	}
	if k[1].Sign() == -1 {
		k[1].Neg(&k[1])
		phiQ.Neg(api, phiQ)
	}
	nbits := k[0].BitLen()
	if k[1].BitLen() > nbits {
		nbits = k[1].BitLen()
	}
	negQ.Neg(api, Q)
	negPhiQ.Neg(api, phiQ)
	var table [4]g2AffP
	table[0] = negQ
	table[1] = Q
	table[2] = negQ
	table[3] = Q

	if cfg.CompleteArithmetic {
		table[0].AddUnified(api, negPhiQ)
		table[1].AddUnified(api, negPhiQ)
		table[2].AddUnified(api, phiQ)
		table[3].AddUnified(api, phiQ)
	} else {
		table[0].AddAssign(api, negPhiQ)
		table[1].AddAssign(api, negPhiQ)
		table[2].AddAssign(api, phiQ)
		table[3].AddAssign(api, phiQ)
	}

	Acc = table[3]
	// if both high bits are set, then we would get to the incomplete part,
	// handle it separately.
	if k[0].Bit(nbits-1) == 1 && k[1].Bit(nbits-1) == 1 {
		if cfg.CompleteArithmetic {
			Acc.AddUnified(api, Acc)
			Acc.AddUnified(api, table[3])
		} else {
			Acc.Double(api, Acc)
			Acc.AddAssign(api, table[3])
		}
		nbits = nbits - 1
	}
	for i := nbits - 1; i > 0; i-- {
		if cfg.CompleteArithmetic {
			Acc.AddUnified(api, Acc)
			Acc.AddUnified(api, table[k[0].Bit(i)+2*k[1].Bit(i)])
		} else {
			Acc.DoubleAndAdd(api, &Acc, &table[k[0].Bit(i)+2*k[1].Bit(i)])
		}
	}

	// i = 0
	if cfg.CompleteArithmetic {
		negQ.AddUnified(api, Acc)
		Acc.Select(api, k[0].Bit(0), Acc, negQ)
		negPhiQ.AddUnified(api, Acc)
	} else {
		negQ.AddAssign(api, Acc)
		Acc.Select(api, k[0].Bit(0), Acc, negQ)
		negPhiQ.AddAssign(api, Acc)
	}
	Acc.Select(api, k[1].Bit(0), Acc, negPhiQ)
	P.X, P.Y = Acc.X, Acc.Y

	return P
}

// Assign a value to self (witness assignment)
func (p *g2AffP) Assign(p1 *bls12377.G2Affine) {
	p.X.Assign(&p1.X)
	p.Y.Assign(&p1.Y)
}

// AssertIsEqual constraint self to be equal to other into the given constraint system
func (p *g2AffP) AssertIsEqual(api frontend.API, other g2AffP) {
	p.X.AssertIsEqual(api, other.X)
	p.Y.AssertIsEqual(api, other.Y)
}

// DoubleAndAdd computes 2*p1+p2 in affine coords
func (p *g2AffP) DoubleAndAdd(api frontend.API, p1, p2 *g2AffP) *g2AffP {

	var n, d, l1, l2, x3, x4, y4 fields_bls12377.E2

	// compute lambda1 = (y2-y1)/(x2-x1)
	n.Sub(api, p1.Y, p2.Y)
	d.Sub(api, p1.X, p2.X)
	l1.DivUnchecked(api, n, d)

	// compute x3 = lambda1**2-x1-x2
	x3.Square(api, l1).
		Sub(api, x3, p1.X).
		Sub(api, x3, p2.X)

	// omit y3 computation
	// compute lambda2 = -lambda1-2*y1/(x3-x1)
	n.Double(api, p1.Y)
	d.Sub(api, x3, p1.X)
	l2.DivUnchecked(api, n, d)
	l2.Add(api, l2, l1).Neg(api, l2)

	// compute x4 =lambda2**2-x1-x3
	x4.Square(api, l2).
		Sub(api, x4, p1.X).
		Sub(api, x4, x3)

	// compute y4 = lambda2*(x1 - x4)-y1
	y4.Sub(api, p1.X, x4).
		Mul(api, l2, y4).
		Sub(api, y4, p1.Y)

	p.X = x4
	p.Y = y4

	return p
}

// ScalarMulBase computes s * g2 and returns it, where g2 is the fixed generator. It doesn't modify s.
func (P *g2AffP) ScalarMulBase(api frontend.API, s frontend.Variable) *g2AffP {

	points := getTwistPoints()

	sBits := api.ToBinary(s, 253)

	var res, tmp g2AffP

	// i = 1, 2
	// gm[0] = 3g, gm[1] = 5g, gm[2] = 7g
	res.X.Lookup2(api, sBits[1], sBits[2],
		fields_bls12377.E2{
			A0: points.G2x[0],
			A1: points.G2x[1]},
		fields_bls12377.E2{
			A0: points.G2m[0][0],
			A1: points.G2m[0][1]},
		fields_bls12377.E2{
			A0: points.G2m[1][0],
			A1: points.G2m[1][1]},
		fields_bls12377.E2{
			A0: points.G2m[2][0],
			A1: points.G2m[2][1]})
	res.Y.Lookup2(api, sBits[1], sBits[2],
		fields_bls12377.E2{
			A0: points.G2y[0],
			A1: points.G2y[1]},
		fields_bls12377.E2{
			A0: points.G2m[0][2],
			A1: points.G2m[0][3]},
		fields_bls12377.E2{
			A0: points.G2m[1][2],
			A1: points.G2m[1][3]},
		fields_bls12377.E2{
			A0: points.G2m[2][2],
			A1: points.G2m[2][3]})

	for i := 3; i < 253; i++ {
		// gm[i] = [2^i]g
		tmp.X = res.X
		tmp.Y = res.Y
		tmp.AddAssign(api, g2AffP{
			fields_bls12377.E2{
				A0: points.G2m[i][0],
				A1: points.G2m[i][1]},
			fields_bls12377.E2{
				A0: points.G2m[i][2],
				A1: points.G2m[i][3]}})
		res.Select(api, sBits[i], tmp, res)
	}

	// i = 0
	tmp.Neg(api, g2AffP{
		fields_bls12377.E2{A0: points.G2x[0], A1: points.G2x[1]},
		fields_bls12377.E2{A0: points.G2y[0], A1: points.G2y[1]}})
	tmp.AddAssign(api, res)
	res.Select(api, sBits[0], res, tmp)

	P.X = res.X
	P.Y = res.Y

	return P
}

func (P *g2AffP) psi(api frontend.API, q *g2AffP) *g2AffP {
	var x, y fields_bls12377.E2
	x.Conjugate(api, q.X)
	x.MulByFp(api, x, "80949648264912719408558363140637477264845294720710499478137287262712535938301461879813459410946")
	y.Conjugate(api, q.Y)
	y.MulByFp(api, y, "216465761340224619389371505802605247630151569547285782856803747159100223055385581585702401816380679166954762214499")

	P.X = x
	P.Y = y

	return P
}
