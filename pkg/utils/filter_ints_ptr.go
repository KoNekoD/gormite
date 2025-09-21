package utils

func FilterIntsPtr(lengths []*int) []int {
	out := make([]int, 0)
	for _, length := range lengths {
		if length != nil {
			out = append(out, *length)
		}
	}
	return out
}
