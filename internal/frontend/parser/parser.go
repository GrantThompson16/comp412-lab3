package parser

import (
	"fmt"
	"io"

	"lab3/internal/frontend/ir"
	"lab3/internal/frontend/scanner"
	"lab3/internal/frontend/token"
)

type Output struct {
	IR        *ir.IR //Intermediate representation - may be partially filled if errors occur
	OpCount   int    // Num of valid operations parsed/added to IR
	HadErrors bool   // True if 1 or more errors occured while parsing, false otherwise
}

type Parser struct {
	S         *scanner.Scanner
	ErrW      io.Writer // Where to print ERROR messages
	hadErrors bool      // flag to indicate errors while parsing
}

// Constructor to create a new perser using the provided scanner and error writer
func New(s *scanner.Scanner, errw io.Writer) *Parser {
	return &Parser{
		S:    s,
		ErrW: errw,
	}
}

// Parse is the main driver of the Parser class, consumes tthe entire token stream, returns an IR
func (p *Parser) Parse() Output {
	output := Output{
		IR: ir.NewIr(),
	}

	// Read input file line by line until EOF token reached
	for {
		tok := p.next()

		switch tok.Operation {
		case token.EOFToken:
			// End of file reached, finalize and return output (IR wrapper)
			output.HadErrors = p.hadErrors
			return output

		case token.EOL:
			// ignore (end of line, or comment, or blank line)
			continue

		// MEMOP (load, store) ex: "load r1 => r2" or in token form "OP_LOAD REG ARROW REG"
		case token.OP_LOAD, token.OP_STORE:
			node, success := p.finishMemOp(tok)
			if success {
				// Successfully parsed full operation, add to IR and increase operation count
				output.IR.Append(node)
				output.OpCount++

				// Ensure line ends with a newline char. expectedNext handles error case if it does not
				// it reports error, and skips to next EOL/EOF token
				p.expectEndOfLine()
			}
			// On failure, finishMemOp already reports error and skips to next EOL/EOF token

		// LOADI ex: "loadI 15 => r1" or in tokwn form "OP_LOADI INT ARROW REG"
		case token.OP_LOADI:
			node, success := p.finishLoadI(tok)
			if success {
				// Successfully parsed full operation, add to IR and increment operation count
				output.IR.Append(node)
				output.OpCount++
				// Ensure line ends with a newline char. expectedNext handles error case if it does not
				// it reports error, and skips to next EOL/EOF token
				p.expectEndOfLine()
			}
			// On failure, finishMemOp already reports error and skips to next EOL/EOF token

		// ARITHOP (add, sub, mult, lshift, rshift). ex: "add r1, r2 => r3" or in token form "OP_ADD REG COMMA REG ARROW REG"
		case token.OP_ADD, token.OP_SUB, token.OP_MULT, token.OP_LSHIFT, token.OP_RSHIFT:
			node, success := p.finishArithOp(tok)
			if success {
				// Successfully parsed full operation, add to IR an increment operation count
				output.IR.Append(node)
				output.OpCount++
				// Ensure line ends with a newline char. expectedNext handles error case if it does not
				// it reports error, and skips to next EOL/EOF token
				p.expectEndOfLine()
			}
			// On failure, finishMemOp already reports error and skips to next EOL/EOF token

		// OUTPUT ex: "output r1" or in token form "OUTPUT REG"
		case token.OP_OUTPUT:
			node, success := p.finishOutput(tok)
			if success {
				// Successfully parsed full operation, add to IR an increment operation count
				output.IR.Append(node)
				output.OpCount++
				// Ensure line ends with a newline char. expectedNext handles error case if it does not
				// it reports error, and skips to next EOL/EOF token
				p.expectEndOfLine()
			}
			// On failure, finishMemOp already reports error and skips to next EOL/EOF token

		// NOP ex: "nop" or in token form "NOP" (idles for one cycle)
		//* Note* cannot fail, no following operands to check
		case token.OP_NOP:
			node := ir.NewIRNode(tok.Operation, tok.Line)
			output.IR.Append(node)
			output.OpCount++
			// Ensure line ends with a newline char. expectedNext handles error case if it does not
			// it reports error, and skips to next EOL/EOF token
			p.expectEndOfLine()

		// Scanner discovered lexical error and already printed it with error writer
		// Skip to the next line
		case token.ILLEGAL:
			p.skipToEOL()

		default:
			// unknown token at start of line (probably wont happen?)
			p.error(tok.Line, "unexpected token at line start")
			p.skipToEOL()
		}
	}
}

/*
===================================
Parsing specific operations helpers
===================================
*/

// MemOp helper  for load/store, ensures load/store correct syntax, reports errors if incorrect syntax.
// If error found, skips/consumes tokens until EOL/EOF token reached as this line contains an error.
// On success, returns the next token and true
// On failure, returns the next token (the token that caused the error) and false
func (p *Parser) finishMemOp(tok token.Tok) (*ir.IRNode, bool) {
	node := ir.NewIRNode(tok.Operation, tok.Line)

	sourceReg, success := p.expectedNext(token.REG, "expected source register (ex: 'r1')")
	// Next token was not a refister
	if !success {
		return nil, false
	}

	// Next token was register, set Source1 field in IRNode to return
	node.SetSource1(sourceReg.Int)

	// Expect next token is ARROW ('=>')
	_, success = p.expectedNext(token.ARROW, "expected '=>' after source register")
	if !success {
		// if next token is not arrow, return false to indicate failure parsing line
		return nil, false
	}

	// Expected next token is destination register REG
	dest, success := p.expectedNext(token.REG, "expected destination register following '=>' (ex: 'r2')")
	if !success {
		// if next token is not register, return false to indicate failure parsing line
		return nil, false
	}
	// Set destination field to register num
	node.SetDest(dest.Int)

	// Return the IR node and true to indicate success
	return node, true
}

// LoadI helper, ensures loadI correct syntax, reports errors if incorrect syntax.
// If error found, skips/consumes tokens until EOL/EOF token reached as this line contains an error.
// On success, returns the next token and true
// On failure, returns the next token (the token that caused the error) and false
func (p *Parser) finishLoadI(tok token.Tok) (*ir.IRNode, bool) {
	node := ir.NewIRNode(tok.Operation, tok.Line)

	// expect next token to be a constant INT
	val, success := p.expectedNext(token.INT, "expected integer constant after loadI")
	if !success {
		// next token was not an int, return false to indicate failure
		return nil, false
	}

	node.SetConst(val.Int) // set Source1 field in IRNode to the constant and sets IsConst to true

	// expect next token is ARROW '=>'
	_, success = p.expectedNext(token.ARROW, "expected '=>' after constant")
	if !success {
		// next token was not an arrow, reutrn false to indicate failure parsing line
		return nil, false
	}

	// expect next token to  be a register (destination register)
	dest, success := p.expectedNext(token.REG, "expected destination register following '=>' (ex: 'r2')")
	if !success {
		// next token was not register, return false to indicate failure parsing line
		return nil, false
	}
	node.SetDest(dest.Int) // set destination field in IRNode

	return node, true
}

// ARITHOP (add, sub, mult, lshift, rshift) helper. ensures correct syntax, reports errors if incorrect syntax
// If error found, skips/consumes tokens until EOL/EOF token reached as this line contains an error.
// On success, returns the next token and true
// On failure, returns the next token (the token that caused the error) and false
func (p *Parser) finishArithOp(tok token.Tok) (*ir.IRNode, bool) {
	node := ir.NewIRNode(tok.Operation, tok.Line)

	// Expect next token to be a register (source1)
	source1, success := p.expectedNext(token.REG, "expected first source register (ex: 'r1')")
	if !success {
		// next token was not a register, incorrect syntax return false
		return nil, false
	}
	node.SetSource1(source1.Int) // Set IRNode source1 to register num

	// Expected next token to be a COMMA (',')
	_, success = p.expectedNext(token.COMMA, "expected ',' between source registers")
	if !success {
		// next token was not a comma, incorrect syntax return false
		return nil, false
	}

	// Expected next token to be a register (source1)
	source2, success := p.expectedNext(token.REG, "expected second source register (ex: 'r2')")
	if !success {
		// next token was not a register, incorrect syntax return false
		return nil, false
	}
	node.SetSource2(source2.Int) // Set IRNode source2 to register num

	// Expected next token to be an ARROW ('=>')
	_, success = p.expectedNext(token.ARROW, "expected '=>' before destination register")
	if !success {
		// next token was not an arrow, incorrect syntax return false
		return nil, false
	}

	// Expected next token to be destination register (REG)
	dest, success := p.expectedNext(token.REG, "expected destination register following '=>' (ex: 'r2')")
	if !success {
		// next token was not a register, incorrect syntax return false
		return nil, false
	}
	node.SetDest(dest.Int) // Set IRNode dest field to register num

	return node, true
}

// OUTPUT helper, ensures correct syntac, reports errors if incorrect syntax
// If error found, skips/consumes tokens until EOL/EOF token reached as this line contains an error.
// On success, returns the next token and true
// On failure, returns the next token (the token that caused the error) and false
func (p *Parser) finishOutput(tok token.Tok) (*ir.IRNode, bool) {
	node := ir.NewIRNode(tok.Operation, tok.Line)

	// Expected next token to be an INT
	val, success := p.expectedNext(token.INT, "expected integer constant after output")
	if !success {
		// next token was not a register, incorrect syntax return false
		return nil, false
	}
	node.SetConst(val.Int) // set Source1 field in IRNode to the constant and sets IsConst to true

	return node, true
}

/*
scanner.Next() wrappers and helpers. These helper methods validate/consume the next token.
*/

// Wrapper around scanner.next()
func (p *Parser) next() token.Tok {
	return p.S.Next()
}

// expectedNext() is wrapper around parser.Next(). The purpose of this method is to consume the next token and verify it with expectedKind.
// If token is not the expected kind, report error with msg and return false
// Use case --> already consumed a load, expectedNext(token.REG, "expected register...")
func (p *Parser) expectedNext(expectedKind token.Operation, msg string) (token.Tok, bool) {
	tok := p.next()

	// Next token is the expected type, return it and mark success (true)
	if tok.Operation == expectedKind {
		return tok, true
	}

	// Next token is not expected type, report error and return failure (false)
	p.error(tok.Line, msg)
	p.skipToEOL()
	return tok, false
}

// expectEndOfLine consumes the next token and accepts either EOL or EOF.
// If it sees anything else, it reports "extra tokens at end of line".
func (p *Parser) expectEndOfLine() {
    tok := p.next()
    switch tok.Operation {
    case token.EOL, token.EOFToken:
        return
    default:
        p.error(tok.Line, "extra tokens at end of line")
        p.skipToEOL()
    }
}

/*
Misc error helpers.
*/

// Helper that is called in case of error. Keeps calling parser.next() until an EOL/EOF token is reached.
// Called on error and skips the rest of the line as current line encountered a syntax issue
func (p *Parser) skipToEOL() {
	for {
		tok := p.next()
		if tok.Operation == token.EOL || tok.Operation == token.EOFToken {
			return
		}
	}
}

// error is a helper method that prints an error message to the ErrW field in the parser.
// Sets the hadErrors flag
func (p *Parser) error(lineNum int, errorMsg string) {
	p.hadErrors = true
	if p.ErrW == nil {
		return
	}
	fmt.Fprintf(p.ErrW, "ERROR %d: %s\n", lineNum, errorMsg)
}
