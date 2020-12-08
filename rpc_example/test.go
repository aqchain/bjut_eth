package main

type Test struct{}

type Args struct {
	S string
}

type Result struct {
	String string
	Int    int
	Args   *Args
}

func (t *Test) Echo(str string, i int, args *Args) Result {
	return Result{str, i, args}
}
