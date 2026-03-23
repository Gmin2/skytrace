package tracker

import "math"

// Kalman filter constants
const (
	sigmaAccel    = 3.0   // process noise: acceleration std dev (m/s²)
	sigmaMlatPos  = 500.0 // MLAT measurement noise std dev (meters)
	sigmaAdsbPos  = 50.0  // ADS-B measurement noise std dev (meters)
	maxHistoryLen = 120   // max trail points to keep
)

// initKalman initializes the Kalman state for a new track.
func initKalman(t *Track, x, y, z float64) {
	t.X, t.Y, t.Z = x, y, z
	t.VX, t.VY, t.VZ = 0, 0, 0

	// Initial covariance: large position uncertainty, very large velocity uncertainty
	for i := 0; i < 6; i++ {
		for j := 0; j < 6; j++ {
			t.P[i][j] = 0
		}
	}
	// Position uncertainty: 1000m
	t.P[0][0] = 1e6
	t.P[1][1] = 1e6
	t.P[2][2] = 1e6
	// Velocity uncertainty: 300m/s (~600kts)
	t.P[3][3] = 9e4
	t.P[4][4] = 9e4
	t.P[5][5] = 9e4
}

// predictKalman propagates the state forward by dt seconds.
func predictKalman(t *Track, dt float64) {
	if dt <= 0 || dt > 300 { // skip if negative or too large
		return
	}

	// State prediction: constant velocity model
	// x_new = x + vx*dt
	t.X += t.VX * dt
	t.Y += t.VY * dt
	t.Z += t.VZ * dt

	// Covariance prediction: P_new = F*P*F' + Q
	// F = [[I, dt*I], [0, I]]
	// Q = sigma_a^2 * [[dt^3/3*I, dt^2/2*I], [dt^2/2*I, dt*I]]

	dt2 := dt * dt
	dt3 := dt2 * dt
	q := sigmaAccel * sigmaAccel

	// Apply F*P*F' (in-place approximation for 6x6)
	var Pnew [6][6]float64

	for i := 0; i < 6; i++ {
		for j := 0; j < 6; j++ {
			Pnew[i][j] = t.P[i][j]
		}
	}

	// P' = F * P * F^T where F has dt in upper-right 3x3 block
	for i := 0; i < 3; i++ {
		for j := 0; j < 6; j++ {
			Pnew[i][j] = t.P[i][j] + dt*t.P[i+3][j]
		}
	}
	// Now apply F^T on columns
	var P2 [6][6]float64
	for i := 0; i < 6; i++ {
		for j := 0; j < 3; j++ {
			P2[i][j] = Pnew[i][j] + dt*Pnew[i][j+3]
		}
		for j := 3; j < 6; j++ {
			P2[i][j] = Pnew[i][j]
		}
	}

	// Add process noise Q
	for i := 0; i < 3; i++ {
		P2[i][i] += q * dt3 / 3
		P2[i][i+3] += q * dt2 / 2
		P2[i+3][i] += q * dt2 / 2
		P2[i+3][i+3] += q * dt
	}

	t.P = P2
}

// updateKalman updates the track with a position measurement.
// measXYZ: measured position in ECEF. sigmaR: measurement noise std dev.
func updateKalman(t *Track, measX, measY, measZ, sigmaR float64) {
	// Measurement model: H = [I_3x3 | 0_3x3]
	// Innovation: y = z - H*x
	y := [3]float64{measX - t.X, measY - t.Y, measZ - t.Z}

	// Innovation covariance: S = H*P*H' + R
	R := sigmaR * sigmaR
	var S [3][3]float64
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			S[i][j] = t.P[i][j]
		}
		S[i][i] += R
	}

	// Kalman gain: K = P*H' * S^-1
	// P*H' is first 3 columns of P (6x3)
	Sinv, ok := invert3x3(S)
	if !ok {
		return // skip update if singular
	}

	// K (6x3) = P[:, 0:3] * Sinv
	var K [6][3]float64
	for i := 0; i < 6; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				K[i][j] += t.P[i][k] * Sinv[k][j]
			}
		}
	}

	// State update: x = x + K*y
	state := [6]float64{t.X, t.Y, t.Z, t.VX, t.VY, t.VZ}
	for i := 0; i < 6; i++ {
		for j := 0; j < 3; j++ {
			state[i] += K[i][j] * y[j]
		}
	}
	t.X, t.Y, t.Z = state[0], state[1], state[2]
	t.VX, t.VY, t.VZ = state[3], state[4], state[5]

	// Covariance update: P = (I - K*H) * P
	var Pnew [6][6]float64
	for i := 0; i < 6; i++ {
		for j := 0; j < 6; j++ {
			Pnew[i][j] = t.P[i][j]
			for k := 0; k < 3; k++ {
				Pnew[i][j] -= K[i][k] * t.P[k][j]
			}
		}
	}
	t.P = Pnew

	// Compute speed and heading from velocity
	// Convert ECEF velocity to approximate ground speed/heading
	speed := math.Sqrt(t.VX*t.VX + t.VY*t.VY + t.VZ*t.VZ)
	t.SpeedKts = speed * 1.94384 // m/s to knots
}

func invert3x3(A [3][3]float64) ([3][3]float64, bool) {
	det := A[0][0]*(A[1][1]*A[2][2]-A[1][2]*A[2][1]) -
		A[0][1]*(A[1][0]*A[2][2]-A[1][2]*A[2][0]) +
		A[0][2]*(A[1][0]*A[2][1]-A[1][1]*A[2][0])

	if math.Abs(det) < 1e-30 {
		return [3][3]float64{}, false
	}

	inv := 1.0 / det
	var B [3][3]float64
	B[0][0] = inv * (A[1][1]*A[2][2] - A[1][2]*A[2][1])
	B[0][1] = inv * (A[0][2]*A[2][1] - A[0][1]*A[2][2])
	B[0][2] = inv * (A[0][1]*A[1][2] - A[0][2]*A[1][1])
	B[1][0] = inv * (A[1][2]*A[2][0] - A[1][0]*A[2][2])
	B[1][1] = inv * (A[0][0]*A[2][2] - A[0][2]*A[2][0])
	B[1][2] = inv * (A[0][2]*A[1][0] - A[0][0]*A[1][2])
	B[2][0] = inv * (A[1][0]*A[2][1] - A[1][1]*A[2][0])
	B[2][1] = inv * (A[0][1]*A[2][0] - A[0][0]*A[2][1])
	B[2][2] = inv * (A[0][0]*A[1][1] - A[0][1]*A[1][0])
	return B, true
}
