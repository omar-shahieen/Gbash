package shell

type RedirMode int

const (
	ModeRead RedirMode = iota
	ModeTruncate
	ModeAppend
)

type Redirection struct {
	FD       int // 0 = stdin, 1 = stdout, 2 = stderr
	Mode     RedirMode
	Filename string
}

type Command struct {
	Name         string
	Args         []string
	Redirections []Redirection
}

type Pipeline struct {
	Commands   []Command
	Background bool
}
