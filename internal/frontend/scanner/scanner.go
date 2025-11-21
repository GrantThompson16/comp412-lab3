package scanner

import (
	"bufio"
	"lab2/internal/frontend/token"
	"fmt"

	//maybe?
	"io"
	"os"
)

type Scanner struct {
	br          *bufio.Reader
	line        int       // 1-based current line number; incremented when we emit EOL
	captureLex  bool      //if True, fill Tok.Lex (for -s printing)
	emitIllegal bool      // If true, return ILLEGAL tokens
	errw        io.Writer // Where to print error msgs (in os.Stderr)
}

const maxInt32 = (1 << 31) - 1 // 2^31-1 upper bound for INT tokens (MAYBE DONT NEED??)

/*
New creates a scanner over io.Reader r.
*/
func New(r io.Reader, captureLex bool, emitIllegal bool) *Scanner {
	return &Scanner{
		br:          bufio.NewReaderSize(r, 4<<10), //4 kib CAN CHANGE LATER
		line:        1,
		captureLex:  captureLex,
		emitIllegal: emitIllegal,
		errw:        os.Stderr,
	}
}

/*
Sets the scanner's error writer (useful for debugging purposes)
*/
func (s *Scanner) SetErrorWriter(w io.Writer) {
	s.errw = w
}

/*
Next() returns the next token in the stream. Continuously called by the parser.
Returns exactly one token per call, leaves reader positioned at the next tokens start.
On the end of a file, returns the EOFToken.
*/
func (s *Scanner) Next() token.Tok {
	for {
		// Read a single byte to classify what comes next
		b, err := s.br.ReadByte()
		if err == io.EOF {
			// No more bytes to read, we have reached the end of the file. Return EOF token
			return token.Tok{
				Operation: token.EOFToken,
				Line:      s.line,
			}
		}

		// Switch statement to switch on the character read by br
		switch b {

		// Spaces and tabs can be ignored
		case ' ', '\t':
			continue

		// ------ END OF LINE HANDLING ------
		// Support \n and \r
		case '\n':
			t := token.Tok{
				Operation: token.EOL,
				Line:      s.line,
			}
			s.line++
			return t

		case '\r':
			//Check if next line is \n, as \r\n acts as a single new line
			next, er := s.br.ReadByte()

			// If next char is not \n, place the pointer back towards before the byte
			if er == nil && next != '\n' {
				_ = s.br.UnreadByte()
			}

			t := token.Tok{
				Operation: token.EOL,
				Line:      s.line,
			}

			s.line++
			return t

		// Comment handeling: '//' to EOL character becomes one single EOL token (we ignore comment text)
		case '/':
			next, er := s.br.ReadByte()
			//If true, this is a comment and eat/discard the rest of the line
			if er == nil && next == '/' {
				for {
					ch, er2 := s.br.ReadByte()
					// If this comment ends as the last line in the file
					if er2 == io.EOF {
						return token.Tok{
							Operation: token.EOFToken,
							Line:      s.line,
						}
					}

					// Reached end of comment, found newline character
					if ch == '\n' {
						t := token.Tok{
							Operation: token.EOL,
							Line:      s.line,
						}
						s.line++
						return t
					}
					if ch == '\r' {
						n2, err3 := s.br.ReadByte()
						if err3 == nil && n2 != '\n' {
							_ = s.br.UnreadByte()
						}
						t := token.Tok{
							Operation: token.EOL,
							Line:      s.line,
						}
						s.line++
						return t
					}
				}
			}
			// Not actually '//' --> stray '/' character is illegal, put the non-'/' back so we don't skip it
			_ = s.br.UnreadByte()

			if s.reportLexError("unexpected '/'") {
				// if emitIllegal true, emit ILLEGAL token so parser can see it
				return s.tok(token.ILLEGAL, "/")
			}
			// Otherwise, skip the word to keep returning valid tokens
			s.skipRestOfLine()
			continue

		// --- Punctutation Handling
		case ',':
			return s.tok(token.COMMA, ",")
		case '=':
			// Only valid when immediately followed by a '>'
			nxt, err := s.br.ReadByte()
			if err == nil && nxt == '>' {
				return s.tok(token.ARROW, "=>")
			} else {
				// Not an arrow token, illegal artiffact, unread byte and report error
				if err == nil {
					_ = s.br.UnreadByte()
				}
				if s.reportLexError("unexpected '=' (did you mean '=>' ?)") {
					return s.tok(token.ILLEGAL, "=")
				}
				s.skipRestOfLine()
				continue
			}
		// Integers, opcodes, registers
		default:
			// Letters either opcode or register
			if isLetter(b) {
				// Checking if the word is a regsiter (r followed by at least one integer)
				if b == 'r' {
					ch, err := s.br.ReadByte()
					if err == nil {
						// Ensure the 'r' is followed by a non-neg integer constant
						if isDigit(ch) { //NEED A CHECK TO ENSURE WITHIN LEGAL INTEGER RANGE???????? MAXINTVAL ********
							// Start to accumulate the rest of the register number
							val := int(ch - '0')
							lex := []byte{'r', ch}
							for {
								d, err2 := s.br.ReadByte()
								if err2 != nil {
									break
								}
								if isDigit(d) {
									val = val*10 + int(d-'0')
									lex = append(lex, d)
								} else {
									_ = s.br.UnreadByte()
									break
								}
							}
							return token.Tok{
								Operation: token.REG,
								Lex:       s.lex(lex), // will be empty in non -s modes
								Int:       val,        //register number
								Line:      s.line,
							}
						}
						// 'r' followed by non-digit is NOT a register (illegal word/artificact or rshift)
						// Push the pointer back so we treat the whole thing as a generic word
						_ = s.br.UnreadByte()
					}
				}

				// Read a letters-only word for opcode recognition (case-sensitive)
				buf := []byte{b}
				for {
					ch, err := s.br.ReadByte()
					if err != nil {
						break
					}
					if isLetter(ch) {
						buf = append(buf, ch)
					} else {
						_ = s.br.UnreadByte()
						break
					}
				}

				// Exact opcdoe match (fast path)
				op, success := keywordOp(buf)
				if success {
					//Enforce opcode followed by blank space
					ch, err := s.br.ReadByte()
					if err == nil {
						if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
							s.reportLexError(fmt.Sprintf("%q is not a valid word.", string([]byte{ch})))
							s.skipRestOfLine()
						} else {
							s.br.UnreadByte()
						}
					}
					return token.Tok{
						Operation: op,
						Lex:       s.lex(buf),
						Line:      s.line,
					}
				}

				// Not exact match, choose longest possible opcode (storeabc -> "store" + "abc", addI -> "add" + "I")
				op, prefLength, success := keywordPrefix(buf)
				if success {
					tok := token.Tok{
						Operation: op,
						Lex:       s.lex(buf[:prefLength]),
						Line:      s.line,
					}
					tail := buf[prefLength:]
					s.reportLexError(fmt.Sprintf("%q is not a valid word.", string(tail)))
					s.skipRestOfLine()
					return tok
				}

				// Not a register or opcode --> lexical error/illegal word like 's12, foo, rx'
				if s.reportLexError(fmt.Sprintf("invalid operation: '%s'", string(buf))) {
					return token.Tok{
						Operation: token.ILLEGAL,
						Lex:       s.lex(buf),
						Line:      s.line,
					}
				}
				s.skipRestOfLine()
				continue
			}
			// ---- Digits: INT ----
			if isDigit(b) {
				val := int(b - '0')
				buf := []byte{b}

				// Read the rest of the integer
				for {
					ch, err := s.br.ReadByte()
					if err != nil {
						break
					}
					if isDigit(ch) {
						val = val*10 + int(ch-'0')
						buf = append(buf, ch)
					} else {
						_ = s.br.UnreadByte()
						break
					}
				}
				return token.Tok{
					Operation: token.INT,
					Lex:       s.lex(buf),
					Int:       val,
					Line:      s.line,
				}
			}

			// Anything else is a lexical error
			if s.reportLexError(fmt.Sprintf("Unexpected character '%q' on line %d", b, s.line)) {
				return s.tok(token.ILLEGAL, string([]byte{b}))
			}
			s.skipRestOfLine()
			continue
		}
	}

}

// tok creates a token.Tok for punctuation/opcode when the literal spelling is known
// When captureLex is false, we avoid string allocation by ensuring Lex:""
func (s *Scanner) tok(op token.Operation, lit string) token.Tok {
	if s.captureLex {
		return token.Tok{
			Operation: op,
			Lex:       lit,
			Line:      s.line,
		}
	}
	return token.Tok{
		Operation: op,
		Line:      s.line,
	}
}

// lex converts a byte slice into a string only if captureLex is true
func (s *Scanner) lex(b []byte) string {
	if s.captureLex {
		return string(b)
	}
	return ""
}

// reportLexError prints lexical error and returns whether the caller should emit an illegal
// token (true) or 'skip' the word that caused said error (false)
func (s *Scanner) reportLexError(msg string) bool {
	fmt.Fprintf(s.errw, "ERROR %d: %s\n", s.line, msg)
	return s.emitIllegal
}

// skipRestOfLine consumes and discards bytes until the end of the line is reached.
// consumes the rest of the line after an illegal token is found, stopping when it reaches a newline character.
func (s *Scanner) skipRestOfLine() {
	for {
		c, er := s.br.ReadByte()
		if er != nil {
			return // EOF: nothing more to skip
		}
		switch c {
		case '\n', '\r':
			_ = s.br.UnreadByte() // let the main loop see this boundary
			return
		}
	}
}

// ASCII Helpers
func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
func isDigit(b byte) bool {
	return (b >= '0' && b <= '9')
}

// keywordOP recognizes the 10 opcodes based on their length, then checking their specific bytes
func keywordOp(b []byte) (token.Operation, bool) {
	switch len(b) {
	case 3: // add, sub, nop
		if b[0] == 'a' && b[1] == 'd' && b[2] == 'd' {
			return token.OP_ADD, true
		}
		if b[0] == 's' && b[1] == 'u' && b[2] == 'b' {
			return token.OP_SUB, true
		}
		if b[0] == 'n' && b[1] == 'o' && b[2] == 'p' {
			return token.OP_NOP, true
		}
	case 4: // load, mult
		if b[0] == 'l' && b[1] == 'o' && b[2] == 'a' && b[3] == 'd' {
			return token.OP_LOAD, true
		}
		if b[0] == 'm' && b[1] == 'u' && b[2] == 'l' && b[3] == 't' {
			return token.OP_MULT, true
		}
	case 5: // store, loadI
		if b[0] == 'l' && b[1] == 'o' && b[2] == 'a' && b[3] == 'd' && b[4] == 'I' {
			return token.OP_LOADI, true
		}
		if b[0] == 's' && b[1] == 't' && b[2] == 'o' && b[3] == 'r' && b[4] == 'e' {
			return token.OP_STORE, true
		}
	case 6: // lshift, rshift, output
		if b[0] == 'l' && b[1] == 's' && b[2] == 'h' && b[3] == 'i' && b[4] == 'f' && b[5] == 't' {
			return token.OP_LSHIFT, true
		}
		if b[0] == 'r' && b[1] == 's' && b[2] == 'h' && b[3] == 'i' && b[4] == 'f' && b[5] == 't' {
			return token.OP_RSHIFT, true
		}
		if b[0] == 'o' && b[1] == 'u' && b[2] == 't' && b[3] == 'p' && b[4] == 'u' && b[5] == 't' {
			return token.OP_OUTPUT, true
		}
	}
	return 0, false
}

// keywordPrefix returns (op, length, true) if buf begins with a valid opcode.
// Longest match wins (so "loadI" beats "load").
func keywordPrefix(buf []byte) (token.Operation, int, bool) {
	if len(buf) >= 6 {
		if buf[0] == 'l' && buf[1] == 's' && buf[2] == 'h' && buf[3] == 'i' && buf[4] == 'f' && buf[5] == 't' {
			return token.OP_LSHIFT, 6, true
		}
		if buf[0] == 'r' && buf[1] == 's' && buf[2] == 'h' && buf[3] == 'i' && buf[4] == 'f' && buf[5] == 't' {
			return token.OP_RSHIFT, 6, true
		}
		if buf[0] == 'o' && buf[1] == 'u' && buf[2] == 't' && buf[3] == 'p' && buf[4] == 'u' && buf[5] == 't' {
			return token.OP_OUTPUT, 6, true
		}
	}
	if len(buf) >= 5 {
		if buf[0] == 'l' && buf[1] == 'o' && buf[2] == 'a' && buf[3] == 'd' && buf[4] == 'I' {
			return token.OP_LOADI, 5, true
		}
		if buf[0] == 's' && buf[1] == 't' && buf[2] == 'o' && buf[3] == 'r' && buf[4] == 'e' {
			return token.OP_STORE, 5, true
		}
	}
	if len(buf) >= 4 {
		if buf[0] == 'l' && buf[1] == 'o' && buf[2] == 'a' && buf[3] == 'd' {
			return token.OP_LOAD, 4, true
		}
		if buf[0] == 'm' && buf[1] == 'u' && buf[2] == 'l' && buf[3] == 't' {
			return token.OP_MULT, 4, true
		}
	}
	if len(buf) >= 3 {
		if buf[0] == 'a' && buf[1] == 'd' && buf[2] == 'd' {
			return token.OP_ADD, 3, true
		}
		if buf[0] == 's' && buf[1] == 'u' && buf[2] == 'b' {
			return token.OP_SUB, 3, true
		}
		if buf[0] == 'n' && buf[1] == 'o' && buf[2] == 'p' {
			return token.OP_NOP, 3, true
		}
	}
	return 0, 0, false
}
