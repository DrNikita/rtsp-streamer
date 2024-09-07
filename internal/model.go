package internal

type Response struct {
	Status       int
	IsConverting bool
	Result       any
	Error        string
}
