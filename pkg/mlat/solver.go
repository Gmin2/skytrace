package mlat

import (
	"fmt"
	"math"
)

const (
	SpeedOfLight = 299792458.0 // meters per second
	MaxIter      = 50
	Tolerance    = 1.0 // convergence tolerance in meters
)

// SolveTDOA solves the TDOA multilateration problem.
// sensors: ECEF positions of N sensors, sensors[0] is reference.
// tdoaSec: N-1 time differences in seconds (relative to sensors[0]).
// guess: initial guess in ECEF.
// Returns estimated position in ECEF.
func SolveTDOA(sensors [][3]float64, tdoaSec []float64, guess [3]float64) (pos [3]float64, residual float64, err error) {
	if len(sensors) < 3 || len(tdoaSec) != len(sensors)-1 {
		return pos, 0, fmt.Errorf("need 3+ sensors and N-1 TDOA values")
	}

	n := len(tdoaSec) // number of equations
	pos = guess
	lambda := 1.0 // LM damping parameter

	for iter := 0; iter < MaxIter; iter++ {
		// Compute residuals and Jacobian
		r := make([]float64, n)
		J := make([][3]float64, n)

		d0 := dist(pos, sensors[0])
		if d0 < 1.0 {
			d0 = 1.0 // avoid division by zero
		}

		for i := 0; i < n; i++ {
			di := dist(pos, sensors[i+1])
			if di < 1.0 {
				di = 1.0
			}

			// Residual: measured TDOA distance - computed distance difference
			r[i] = (di - d0) - SpeedOfLight*tdoaSec[i]

			// Jacobian row
			for k := 0; k < 3; k++ {
				J[i][k] = (pos[k]-sensors[i+1][k])/di - (pos[k]-sensors[0][k])/d0
			}
		}

		// Compute J^T * J (3x3) and J^T * r (3x1)
		var JtJ [3][3]float64
		var Jtr [3]float64
		for i := 0; i < n; i++ {
			for a := 0; a < 3; a++ {
				Jtr[a] += J[i][a] * r[i]
				for b := 0; b < 3; b++ {
					JtJ[a][b] += J[i][a] * J[i][b]
				}
			}
		}

		// Add LM damping: (J^T*J + lambda*I)
		for k := 0; k < 3; k++ {
			JtJ[k][k] += lambda
		}

		// Solve 3x3 system: JtJ * delta = -Jtr
		delta, ok := solve3x3(JtJ, [3]float64{-Jtr[0], -Jtr[1], -Jtr[2]})
		if !ok {
			return pos, 0, fmt.Errorf("singular matrix at iteration %d", iter)
		}

		// Check step size
		stepSize := math.Sqrt(delta[0]*delta[0] + delta[1]*delta[1] + delta[2]*delta[2])

		// Compute new position and cost
		newPos := [3]float64{pos[0] + delta[0], pos[1] + delta[1], pos[2] + delta[2]}

		oldCost := costFunction(r)
		newR := computeResiduals(newPos, sensors, tdoaSec)
		newCost := costFunction(newR)

		if newCost < oldCost {
			pos = newPos
			lambda /= 10
			if stepSize < Tolerance {
				return pos, math.Sqrt(newCost/float64(n)), nil
			}
		} else {
			lambda *= 10
			if lambda > 1e12 {
				return pos, math.Sqrt(oldCost/float64(n)), fmt.Errorf("failed to converge after %d iterations", iter)
			}
		}
	}

	finalR := computeResiduals(pos, sensors, tdoaSec)
	return pos, math.Sqrt(costFunction(finalR)/float64(n)), fmt.Errorf("max iterations reached")
}

func computeResiduals(pos [3]float64, sensors [][3]float64, tdoaSec []float64) []float64 {
	d0 := dist(pos, sensors[0])
	r := make([]float64, len(tdoaSec))
	for i := 0; i < len(tdoaSec); i++ {
		di := dist(pos, sensors[i+1])
		r[i] = (di - d0) - SpeedOfLight*tdoaSec[i]
	}
	return r
}

func costFunction(r []float64) float64 {
	var sum float64
	for _, v := range r {
		sum += v * v
	}
	return sum
}

func dist(a, b [3]float64) float64 {
	dx := a[0] - b[0]
	dy := a[1] - b[1]
	dz := a[2] - b[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// solve3x3 solves A*x = b using Cramer's rule.
func solve3x3(A [3][3]float64, b [3]float64) (x [3]float64, ok bool) {
	det := A[0][0]*(A[1][1]*A[2][2]-A[1][2]*A[2][1]) -
		A[0][1]*(A[1][0]*A[2][2]-A[1][2]*A[2][0]) +
		A[0][2]*(A[1][0]*A[2][1]-A[1][1]*A[2][0])

	if math.Abs(det) < 1e-20 {
		return x, false
	}

	invDet := 1.0 / det

	x[0] = invDet * (b[0]*(A[1][1]*A[2][2]-A[1][2]*A[2][1]) -
		A[0][1]*(b[1]*A[2][2]-A[1][2]*b[2]) +
		A[0][2]*(b[1]*A[2][1]-A[1][1]*b[2]))

	x[1] = invDet * (A[0][0]*(b[1]*A[2][2]-A[1][2]*b[2]) -
		b[0]*(A[1][0]*A[2][2]-A[1][2]*A[2][0]) +
		A[0][2]*(A[1][0]*b[2]-b[1]*A[2][0]))

	x[2] = invDet * (A[0][0]*(A[1][1]*b[2]-b[1]*A[2][1]) -
		A[0][1]*(A[1][0]*b[2]-b[1]*A[2][0]) +
		b[0]*(A[1][0]*A[2][1]-A[1][1]*A[2][0]))

	return x, true
}
