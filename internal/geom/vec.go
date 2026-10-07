// Package geom has the small 2D vector type used everywhere.
package geom

import "math"

// Vec is a 2D point or direction in pixels. +Y points down (screen space).
type Vec struct{ X, Y float64 }

func V(x, y float64) Vec { return Vec{x, y} }

func (a Vec) Add(b Vec) Vec             { return Vec{a.X + b.X, a.Y + b.Y} }
func (a Vec) Sub(b Vec) Vec             { return Vec{a.X - b.X, a.Y - b.Y} }
func (a Vec) Scale(k float64) Vec       { return Vec{a.X * k, a.Y * k} }
func (a Vec) Len() float64              { return math.Hypot(a.X, a.Y) }
func (a Vec) Dist(b Vec) float64        { return a.Sub(b).Len() }
func (a Vec) Angle() float64            { return math.Atan2(a.Y, a.X) }
func (a Vec) Lerp(b Vec, k float64) Vec { return a.Add(b.Sub(a).Scale(k)) }

// Norm returns the unit vector, or zero for a zero vector.
func (a Vec) Norm() Vec {
	l := a.Len()
	if l == 0 {
		return Vec{}
	}
	return a.Scale(1 / l)
}

// FromAngle returns the unit vector pointing at angle a (radians).
func FromAngle(a float64) Vec { return Vec{math.Cos(a), math.Sin(a)} }

// Rotate rotates v by a radians.
func (a Vec) Rotate(ang float64) Vec {
	c, s := math.Cos(ang), math.Sin(ang)
	return Vec{a.X*c - a.Y*s, a.X*s + a.Y*c}
}
