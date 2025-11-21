package token

type Operation int

const (
	// Control Flow
	ILLEGAL = iota
	EOL
	EOFToken

	// Opcodes
	OP_LOAD   // "load"
	OP_LOADI  // "loadi"
	OP_STORE  // "store"
	OP_ADD    // "add"
	OP_SUB    // "sub"
	OP_MULT   // "mult"
	OP_LSHIFT // "lshift"
	OP_RSHIFT // "rshift"
	OP_OUTPUT // "output"
	OP_NOP    // "nop"

	// Punctiation
	COMMA // ","
	ARROW // "=>"

	// Operands
	REG // r<non-neg int>
	INT // 0, 1, 2 ...
)

// String() Method is used by the -s flag, allowing printable human-readable version of the operations
// String() implements Go fmt Stringer() interface allowing easily Go to automatically call String() on our Operation type in fmt.Print, fmt.SPrintf, etc.
func (o Operation) String() string {
	switch o {
	case ILLEGAL:
		return "illegal"
	case EOL:
		return "\\n"
	case EOFToken:
		return "EOF"
	case OP_LOAD:
		return "load"
	case OP_LOADI:
		return "loadI"
	case OP_STORE:
		return "store"
	case OP_ADD:
		return "add"
	case OP_SUB:
		return "sub"
	case OP_MULT:
		return "mult"
	case OP_LSHIFT:
		return "lshift"
	case OP_RSHIFT:
		return "rshift"
	case OP_OUTPUT:
		return "output"
	case OP_NOP:
		return "nop"
	case COMMA:
		return ","
	case ARROW:
		return "=>"
	case REG:
		return "REG"
	case INT:
		return "INT"
	default:
		return "?"
	}
}

// Token represents a single token read from the input file.
type Tok struct {
	Operation Operation
	Lex       string
	Int       int // Numeric value for INT, register number for REG
	Line      int // 1-based line at which the token resides.
}
