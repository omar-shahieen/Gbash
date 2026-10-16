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
	// InheritHistory is set only for an internal background-chain child.
	InheritHistory bool `json:"-"`
	Name           string
	Args           []string
	Redirections   []Redirection
}

type Pipeline struct {
	Commands    []Command
	Background  bool
	CommandText string
}
