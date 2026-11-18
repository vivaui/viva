package base

func pointInRect(x, y float32, r Rect) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}
