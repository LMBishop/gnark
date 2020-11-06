// Copyright 2020 ConsenSys AG
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by gnark/internal/generators DO NOT EDIT

package groth16

import (
	"bytes"
	"math/big"
	"reflect"

	"github.com/consensys/gnark/internal/backend/bls377/fft"
	curve "github.com/consensys/gurvy/bls377"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/prop"

	"testing"
)

func TestProofSerialization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 1000

	properties := gopter.NewProperties(parameters)

	properties.Property("Proof -> writer -> reader -> Proof should stay constant", prop.ForAll(
		func(ar, krs curve.G1Affine, bs curve.G2Affine) bool {
			var proof, pCompressed, pRaw Proof

			// create a random proof
			proof.Ar = ar
			proof.Krs = krs
			proof.Bs = bs

			var bufCompressed bytes.Buffer
			written, err := proof.WriteTo(&bufCompressed)
			if err != nil {
				return false
			}

			read, err := pCompressed.ReadFrom(&bufCompressed)
			if err != nil {
				return false
			}

			if read != written {
				return false
			}

			var bufRaw bytes.Buffer
			written, err = proof.WriteRawTo(&bufRaw)
			if err != nil {
				return false
			}

			read, err = pRaw.ReadFrom(&bufRaw)
			if err != nil {
				return false
			}

			if read != written {
				return false
			}

			return reflect.DeepEqual(&proof, &pCompressed) && reflect.DeepEqual(&proof, &pRaw)
		},
		GenG1(),
		GenG1(),
		GenG2(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProvingKeySerialization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 10

	properties := gopter.NewProperties(parameters)

	properties.Property("ProvingKey -> writer -> reader -> ProvingKey should stay constant", prop.ForAll(
		func(p1 curve.G1Affine, p2 curve.G2Affine) bool {
			var pk, pkCompressed, pkRaw ProvingKey

			// create a random pk
			domain := fft.NewDomain(8)
			pk.Domain = *domain

			pk.NbWires = 6
			pk.NbPrivateWires = 4

			// allocate our slices
			pk.G1.A = make([]curve.G1Affine, pk.NbWires)
			pk.G1.B = make([]curve.G1Affine, pk.NbWires)
			pk.G1.K = make([]curve.G1Affine, pk.NbPrivateWires)
			pk.G1.Z = make([]curve.G1Affine, pk.Domain.Cardinality)
			pk.G2.B = make([]curve.G2Affine, pk.NbWires)

			pk.G1.Alpha = p1
			pk.G2.Beta = p2
			pk.G1.K[1] = p1
			pk.G1.B[0] = p1
			pk.G2.B[0] = p2

			var bufCompressed bytes.Buffer
			written, err := pk.WriteTo(&bufCompressed)
			if err != nil {
				return false
			}

			read, err := pkCompressed.ReadFrom(&bufCompressed)
			if err != nil {
				return false
			}

			if read != written {
				return false
			}

			var bufRaw bytes.Buffer
			written, err = pk.WriteRawTo(&bufRaw)
			if err != nil {
				return false
			}

			read, err = pkRaw.ReadFrom(&bufRaw)
			if err != nil {
				return false
			}

			if read != written {
				return false
			}

			return reflect.DeepEqual(&pk, &pkCompressed) && reflect.DeepEqual(&pk, &pkRaw)
		},
		GenG1(),
		GenG2(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func GenG1() gopter.Gen {
	_, _, g1GenAff, _ := curve.Generators()
	return func(genParams *gopter.GenParameters) *gopter.GenResult {
		var scalar big.Int
		scalar.SetUint64(genParams.NextUint64())

		var g1 curve.G1Affine
		g1.ScalarMultiplication(&g1GenAff, &scalar)

		genResult := gopter.NewGenResult(g1, gopter.NoShrinker)
		return genResult
	}
}

func GenG2() gopter.Gen {
	_, _, _, g2GenAff := curve.Generators()
	return func(genParams *gopter.GenParameters) *gopter.GenResult {
		var scalar big.Int
		scalar.SetUint64(genParams.NextUint64())

		var g2 curve.G2Affine
		g2.ScalarMultiplication(&g2GenAff, &scalar)

		genResult := gopter.NewGenResult(g2, gopter.NoShrinker)
		return genResult
	}
}
