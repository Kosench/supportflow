package clock

import "time"

type System struct {
}

func (s System) Now() time.Time {
	return time.Now().UTC()
}
