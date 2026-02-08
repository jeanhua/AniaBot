package functool

type OptionFuncs struct {
	SendText  func(text string) bool
	SendImage func(url string) bool
}
