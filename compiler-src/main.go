package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type Tok struct {
	K, S string
	L, C int
}
type Lexer struct {
	s       []rune
	i, l, c int
}

func lex(src string) ([]Tok, error) {
	x := &Lexer{s: []rune(src), l: 1, c: 1}
	var o []Tok
	for x.i < len(x.s) {
		r := x.s[x.i]
		if unicode.IsSpace(r) {
			x.adv()
			continue
		}
		if r == '/' && x.peek(1) == '/' {
			for x.i < len(x.s) && x.s[x.i] != '\n' {
				x.adv()
			}
			continue
		}
		if r == '/' && x.peek(1) == '*' {
			x.adv()
			x.adv()
			for x.i < len(x.s) && !(x.s[x.i] == '*' && x.peek(1) == '/') {
				x.adv()
			}
			if x.i >= len(x.s) {
				return nil, fmt.Errorf("%d:%d: unterminated block comment", x.l, x.c)
			}
			x.adv()
			x.adv()
			continue
		}
		l, c := x.l, x.c
		if unicode.IsLetter(r) || r == '_' {
			var b strings.Builder
			for x.i < len(x.s) && (unicode.IsLetter(x.s[x.i]) || unicode.IsDigit(x.s[x.i]) || x.s[x.i] == '_') {
				b.WriteRune(x.s[x.i])
				x.adv()
			}
			o = append(o, Tok{"id", b.String(), l, c})
			continue
		}
		if unicode.IsDigit(r) {
			var b strings.Builder
			dot := false
			for x.i < len(x.s) && (unicode.IsDigit(x.s[x.i]) || (!dot && x.s[x.i] == '.')) {
				if x.s[x.i] == '.' {
					dot = true
				}
				b.WriteRune(x.s[x.i])
				x.adv()
			}
			o = append(o, Tok{"num", b.String(), l, c})
			continue
		}
		if r == '"' || r == '\'' {
			q := r
			x.adv()
			var b strings.Builder
			closed := false
			for x.i < len(x.s) {
				r = x.s[x.i]
				if r == q {
					x.adv()
					closed = true
					break
				}
				if r == '\n' {
					return nil, fmt.Errorf("%d:%d: unterminated string literal", l, c)
				}
				if r == '\\' {
					x.adv()
					if x.i >= len(x.s) {
						break
					}
					m := map[rune]rune{'n': '\n', 't': '\t', 'r': '\r', '\\': '\\', '"': '"', '\'': '\''}
					if v, ok := m[x.s[x.i]]; ok {
						b.WriteRune(v)
					} else {
						b.WriteRune(x.s[x.i])
					}
					x.adv()
				} else {
					b.WriteRune(r)
					x.adv()
				}
			}
			if !closed {
				return nil, fmt.Errorf("%d:%d: unterminated string literal", l, c)
			}
			k := "str"
			if q == '\'' {
				k = "char"
			}
			o = append(o, Tok{k, b.String(), l, c})
			continue
		}
		two := string([]rune{r, x.peek(1)})
		if map[string]bool{"==": true, "!=": true, "<=": true, ">=": true, "&&": true, "||": true, "++": true, "--": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true, "->": true, "=>": true, "::": true}[two] {
			o = append(o, Tok{"sym", two, l, c})
			x.adv()
			x.adv()
			continue
		}
		if strings.ContainsRune("+-*/%(){}[];,.=<>!&|?:", r) {
			o = append(o, Tok{"sym", string(r), l, c})
			x.adv()
			continue
		}
		return nil, fmt.Errorf("%d:%d: unexpected character %q", l, c, r)
	}
	o = append(o, Tok{"eof", "", x.l, x.c})
	return o, nil
}
func (x *Lexer) peek(n int) rune {
	if x.i+n >= len(x.s) {
		return 0
	}
	return x.s[x.i+n]
}
func (x *Lexer) adv() {
	if x.s[x.i] == '\n' {
		x.l++
		x.c = 1
	} else {
		x.c++
	}
	x.i++
}

type Expr interface{}
type Lit struct{ V any }
type Name struct{ N string }
type Unary struct {
	Op string
	X  Expr
}
type Binary struct {
	Op   string
	A, B Expr
}
type Call struct {
	F    Expr
	Args []Expr
}
type Member struct {
	O Expr
	N string
}
type Index struct{ O, I Expr }
type Array struct{ X []Expr }
type NewObj struct {
	N    string
	Args []Expr
}
type Lambda struct {
	Params []Param
	Body   []Stmt
}
type Stmt interface{}
type Block struct{ S []Stmt }
type Var struct {
	Mod, Type, N string
	X            Expr
}
type ExprStmt struct{ X Expr }
type If struct {
	C    Expr
	T, E Stmt
}
type While struct {
	C Expr
	B Stmt
}
type For struct {
	Init Stmt
	C    Expr
	Post Expr
	B    Stmt
}
type Foreach struct {
	Type, N string
	X       Expr
	B       Stmt
}
type Return struct{ X Expr }
type Break struct{}
type Continue struct{}
type Switch struct {
	X     Expr
	Cases []Case
}
type Case struct {
	X   Expr
	B   []Stmt
	Def bool
}
type Param struct {
	Type, N string
	Ref     bool
}
type Fn struct {
	Mod, Name, Ret, Owner                      string
	Params                                     []Param
	Body                                       []Stmt
	Latches                                    []string
	Static, Virtual, Override, Abstract, Final bool
}
type Field struct {
	Mod, Type, N  string
	Init          Expr
	Static, Const bool
}
type Class struct {
	Mod, Name, Base string
	Interfaces      []string
	Fields          []Field
	Methods         map[string][]*Fn
	Abstract, Final bool
}
type Interface struct {
	Name    string
	Methods map[string][]*Fn
}
type Enum struct {
	Name   string
	Values []string
}
type Program struct {
	Fns        map[string][]*Fn
	Classes    map[string]*Class
	Interfaces map[string]*Interface
	Enums      map[string]*Enum
}

type Parser struct {
	t []Tok
	i int
	p *Program
}

func parse(ts []Tok) (*Program, error) {
	p := &Parser{t: ts, p: &Program{map[string][]*Fn{}, map[string]*Class{}, map[string]*Interface{}, map[string]*Enum{}}}
	for !p.is("eof", "") {
		if p.word("import") || p.word("package") || p.word("namespace") {
			for !p.sym(";") && !p.is("eof", "") {
				p.i++
			}
			p.takeSym(";")
			continue
		}
		mods := p.mods()
		if p.word("class") {
			if err := p.class(mods); err != nil {
				return nil, err
			}
			continue
		}
		if p.word("interface") {
			if err := p.iface(); err != nil {
				return nil, err
			}
			continue
		}
		if p.word("enum") {
			if err := p.enum(); err != nil {
				return nil, err
			}
			continue
		}
		f, err := p.fn(mods)
		if err != nil {
			return nil, err
		}
		p.p.Fns[f.Name] = append(p.p.Fns[f.Name], f)
	}
	return p.p, nil
}
func (p *Parser) mods() map[string]bool {
	m := map[string]bool{}
	for p.is("id", "") {
		s := p.cur().S
		if !strings.Contains(" public private protected internal static const abstract final virtual override ", " "+s+" ") {
			break
		}
		m[s] = true
		p.i++
	}
	return m
}
func modstr(m map[string]bool) string {
	for _, x := range []string{"public", "protected", "internal", "private"} {
		if m[x] {
			return x
		}
	}
	return "internal"
}
func (p *Parser) class(m map[string]bool) error {
	p.i++
	n, err := p.ident()
	if err != nil {
		return err
	}
	c := &Class{Mod: modstr(m), Name: n, Methods: map[string][]*Fn{}, Abstract: m["abstract"], Final: m["final"]}
	if p.word("extends") {
		p.i++
		c.Base, _ = p.ident()
	}
	if p.word("implements") {
		p.i++
		for {
			q, e := p.ident()
			if e != nil {
				return e
			}
			c.Interfaces = append(c.Interfaces, q)
			if !p.takeSym(",") {
				break
			}
		}
	}
	if !p.takeSym("{") {
		return p.err("expected '{'")
	}
	for !p.takeSym("}") {
		mm := p.mods()
		if p.word("class") {
			if p.i+1 >= len(p.t) {
				return p.err("expected nested class name")
			}
			innerName := p.t[p.i+1].S
			if err := p.class(mm); err != nil {
				return err
			}
			if inner := p.p.Classes[innerName]; inner != nil {
				p.p.Classes[c.Name+"."+innerName] = inner
			}
			continue
		}
		// Constructor: ClassName(...) { ... } -- no return type.
		if p.is("id", c.Name) && p.i+1 < len(p.t) && p.t[p.i+1].S == "(" {
			p.i++
			p.i++
			f, e := p.fnTail(mm, "void", c.Name, true)
			if e != nil {
				return e
			}
			f.Owner = c.Name
			c.Methods[c.Name] = append(c.Methods[c.Name], f)
			continue
		}
		typ, e := p.typeName()
		if e != nil {
			return e
		}
		name, e := p.ident()
		if e != nil {
			return e
		}
		if p.takeSym("(") {
			f, e := p.fnTail(mm, typ, name, true)
			if e != nil {
				return e
			}
			f.Owner = c.Name
			c.Methods[name] = append(c.Methods[name], f)
		} else {
			var x Expr
			if p.takeSym("=") {
				x, e = p.expr()
				if e != nil {
					return e
				}
			}
			if !p.takeSym(";") {
				return p.err("expected ';' after field")
			}
			c.Fields = append(c.Fields, Field{modstr(mm), typ, name, x, mm["static"], mm["const"]})
		}
	}
	p.p.Classes[n] = c
	return nil
}
func (p *Parser) iface() error {
	p.i++
	n, e := p.ident()
	if e != nil {
		return e
	}
	q := &Interface{Name: n, Methods: map[string][]*Fn{}}
	if !p.takeSym("{") {
		return p.err("expected '{'")
	}
	for !p.takeSym("}") {
		m := p.mods()
		f, e := p.fn(m)
		if e != nil {
			return e
		}
		q.Methods[f.Name] = append(q.Methods[f.Name], f)
	}
	p.p.Interfaces[n] = q
	return nil
}
func (p *Parser) enum() error {
	p.i++
	n, e := p.ident()
	if e != nil {
		return e
	}
	if !p.takeSym("{") {
		return p.err("expected '{'")
	}
	q := &Enum{Name: n}
	for !p.takeSym("}") {
		v, e := p.ident()
		if e != nil {
			return e
		}
		q.Values = append(q.Values, v)
		p.takeSym(",")
	}
	p.takeSym(";")
	p.p.Enums[n] = q
	return nil
}
func (p *Parser) fn(m map[string]bool) (*Fn, error) {
	ret, e := p.typeName()
	if e != nil {
		return nil, e
	}
	n, e := p.ident()
	if e != nil {
		return nil, e
	}
	if !p.takeSym("(") {
		return nil, p.err("expected '('")
	}
	return p.fnTail(m, ret, n, false)
}
func (p *Parser) fnTail(m map[string]bool, ret, n string, opened bool) (*Fn, error) {
	f := &Fn{Mod: modstr(m), Name: n, Ret: ret, Static: m["static"], Virtual: m["virtual"], Override: m["override"], Abstract: m["abstract"], Final: m["final"]}
	if !p.takeSym(")") {
		for {
			typ, e := p.typeName()
			if e != nil {
				return nil, e
			}
			ref := p.takeSym("&")
			pn, e := p.ident()
			if e != nil {
				return nil, e
			}
			f.Params = append(f.Params, Param{typ, pn, ref})
			if p.takeSym(")") {
				break
			}
			if !p.takeSym(",") {
				return nil, p.err("expected ','")
			}
		}
	}
	if p.word("latch") {
		p.i++
		if !p.takeSym("(") {
			return nil, p.err("expected '(' after latch")
		}
		for !p.takeSym(")") {
			var b strings.Builder
			for !p.sym(",") && !p.sym(")") {
				b.WriteString(p.cur().S)
				p.i++
			}
			f.Latches = append(f.Latches, b.String())
			p.takeSym(",")
		}
	}
	if p.takeSym(";") {
		f.Abstract = true
		return f, nil
	}
	b, e := p.block()
	if e != nil {
		return nil, e
	}
	f.Body = b.S
	return f, nil
}
func (p *Parser) block() (*Block, error) {
	if !p.takeSym("{") {
		return nil, p.err("expected '{'")
	}
	var s []Stmt
	for !p.takeSym("}") {
		x, e := p.stmt()
		if e != nil {
			return nil, e
		}
		s = append(s, x)
	}
	return &Block{s}, nil
}
func (p *Parser) stmt() (Stmt, error) {
	if p.sym("{") {
		return p.block()
	}
	if p.word("if") {
		p.i++
		p.takeSym("(")
		c, e := p.expr()
		if e != nil {
			return nil, e
		}
		if !p.takeSym(")") {
			return nil, p.err("expected ')'")
		}
		t, e := p.stmt()
		if e != nil {
			return nil, e
		}
		var z Stmt
		if p.word("else") {
			p.i++
			z, e = p.stmt()
		}
		return &If{c, t, z}, e
	}
	if p.word("while") {
		p.i++
		p.takeSym("(")
		c, e := p.expr()
		if e != nil {
			return nil, e
		}
		p.takeSym(")")
		b, e := p.stmt()
		return &While{c, b}, e
	}
	if p.word("for") {
		p.i++
		p.takeSym("(")
		var init Stmt
		var e error
		if !p.sym(";") {
			if p.looksVar() {
				init, e = p.varstmt()
			} else {
				x, _ := p.expr()
				p.takeSym(";")
				init = &ExprStmt{x}
			}
		} else {
			p.i++
		}
		if e != nil {
			return nil, e
		}
		var c Expr
		if !p.sym(";") {
			c, e = p.expr()
		}
		p.takeSym(";")
		var post Expr
		if !p.sym(")") {
			post, e = p.expr()
		}
		p.takeSym(")")
		b, e := p.stmt()
		return &For{init, c, post, b}, e
	}
	if p.word("foreach") {
		p.i++
		if !p.takeSym("(") {
			return nil, p.err("expected '(' after foreach")
		}
		typ, er := p.typeName()
		if er != nil {
			return nil, er
		}
		n, er := p.ident()
		if er != nil {
			return nil, er
		}
		if !p.word("in") {
			return nil, p.err("expected 'in' in foreach")
		}
		p.i++
		x, er := p.expr()
		if er != nil {
			return nil, er
		}
		if !p.takeSym(")") {
			return nil, p.err("expected ')' after foreach")
		}
		b, er := p.stmt()
		return &Foreach{typ, n, x, b}, er
	}
	if p.word("return") {
		p.i++
		var x Expr
		var e error
		if !p.sym(";") {
			x, e = p.expr()
		}
		p.takeSym(";")
		return &Return{x}, e
	}
	if p.word("break") {
		p.i++
		p.takeSym(";")
		return Break{}, nil
	}
	if p.word("continue") {
		p.i++
		p.takeSym(";")
		return Continue{}, nil
	}
	if p.looksVar() {
		return p.varstmt()
	}
	// A bare type token is almost certainly an incomplete declaration (for example `int;`).
	// Reject it during parsing so editor diagnostics and `check` catch it before execution.
	if p.is("id", "") {
		s := p.cur().S
		if (s == "int" || s == "float" || s == "bool" || s == "string" || s == "char" || s == "void") && p.i+1 < len(p.t) && p.t[p.i+1].S == ";" {
			return nil, p.err("expected variable name after type '" + s + "'")
		}
	}
	x, e := p.expr()
	if e != nil {
		return nil, e
	}
	if !p.takeSym(";") {
		return nil, p.err("expected ';'")
	}
	return &ExprStmt{x}, nil
}
func (p *Parser) looksVar() bool {
	if !p.is("id", "") {
		return false
	}
	j := p.i + 1
	for j+1 < len(p.t) && p.t[j].S == "." && p.t[j+1].K == "id" {
		j += 2
	}
	if j < len(p.t) && p.t[j].S == "<" {
		depth := 0
		for j < len(p.t) {
			if p.t[j].S == "<" {
				depth++
			}
			if p.t[j].S == ">" {
				depth--
				if depth == 0 {
					j++
					break
				}
			}
			j++
		}
	}
	for j+1 < len(p.t) && p.t[j].S == "[" && p.t[j+1].S == "]" {
		j += 2
	}
	return j < len(p.t) && p.t[j].K == "id"
}
func (p *Parser) varstmt() (Stmt, error) {
	m := p.mods()
	typ, e := p.typeName()
	if e != nil {
		return nil, e
	}
	n, e := p.ident()
	if e != nil {
		return nil, e
	}
	var x Expr
	if p.takeSym("=") {
		x, e = p.expr()
	}
	if !p.takeSym(";") {
		return nil, p.err("expected ';'")
	}
	return &Var{modstr(m), typ, n, x}, e
}

var prec = map[string]int{"=": 1, "+=": 1, "-=": 1, "*=": 1, "/=": 1, "||": 2, "&&": 3, "==": 4, "!=": 4, "<": 5, "<=": 5, ">": 5, ">=": 5, "+": 6, "-": 6, "*": 7, "/": 7, "%": 7}

func (p *Parser) expr() (Expr, error) { return p.bin(1) }
func (p *Parser) bin(min int) (Expr, error) {
	a, e := p.unary()
	if e != nil {
		return nil, e
	}
	for {
		op := p.cur().S
		pr := prec[op]
		if pr < min {
			break
		}
		p.i++
		b, e := p.bin(pr + 1)
		if e != nil {
			return nil, e
		}
		a = &Binary{op, a, b}
	}
	return a, nil
}
func (p *Parser) unary() (Expr, error) {
	if p.sym("!") || p.sym("-") || p.sym("+") {
		op := p.cur().S
		p.i++
		x, e := p.unary()
		return &Unary{op, x}, e
	}
	return p.post()
}
func (p *Parser) post() (Expr, error) {
	x, e := p.primary()
	if e != nil {
		return nil, e
	}
	for {
		if p.takeSym("(") {
			var a []Expr
			if !p.takeSym(")") {
				for {
					q, e := p.expr()
					if e != nil {
						return nil, e
					}
					a = append(a, q)
					if p.takeSym(")") {
						break
					}
					p.takeSym(",")
				}
			}
			x = &Call{x, a}
			continue
		}
		if p.takeSym(".") {
			n, e := p.ident()
			if e != nil {
				return nil, e
			}
			x = &Member{x, n}
			continue
		}
		if p.takeSym("[") {
			i, e := p.expr()
			if e != nil {
				return nil, e
			}
			p.takeSym("]")
			x = &Index{x, i}
			continue
		}
		if p.sym("++") || p.sym("--") {
			op := p.cur().S
			p.i++
			aop := "+="
			if op == "--" {
				aop = "-="
			}
			x = &Binary{aop, x, &Lit{1}}
			continue
		}
		break
	}
	return x, nil
}
func (p *Parser) primary() (Expr, error) {
	t := p.cur()
	if t.K == "num" {
		p.i++
		if strings.Contains(t.S, ".") {
			v, _ := strconv.ParseFloat(t.S, 64)
			return &Lit{v}, nil
		}
		v, _ := strconv.Atoi(t.S)
		return &Lit{v}, nil
	}
	if t.K == "str" || t.K == "char" {
		p.i++
		return &Lit{t.S}, nil
	}
	if p.word("true") {
		p.i++
		return &Lit{true}, nil
	}
	if p.word("false") {
		p.i++
		return &Lit{false}, nil
	}
	if p.word("null") {
		p.i++
		return &Lit{nil}, nil
	}
	if p.word("new") {
		p.i++
		n, e := p.typeName()
		if e != nil {
			return nil, e
		}
		p.takeSym("(")
		var a []Expr
		if !p.takeSym(")") {
			for {
				q, _ := p.expr()
				a = append(a, q)
				if p.takeSym(")") {
					break
				}
				p.takeSym(",")
			}
		}
		return &NewObj{n, a}, nil
	}
	if p.takeSym("[") {
		var a []Expr
		if !p.takeSym("]") {
			for {
				q, e := p.expr()
				if e != nil {
					return nil, e
				}
				a = append(a, q)
				if p.takeSym("]") {
					break
				}
				p.takeSym(",")
			}
		}
		return &Array{a}, nil
	}
	if p.takeSym("(") {
		x, e := p.expr()
		p.takeSym(")")
		return x, e
	}
	if t.K == "id" {
		p.i++
		return &Name{t.S}, nil
	}
	return nil, p.err("expected expression")
}
func (p *Parser) cur() Tok            { return p.t[p.i] }
func (p *Parser) is(k, s string) bool { t := p.cur(); return t.K == k && (s == "" || t.S == s) }
func (p *Parser) word(s string) bool  { return p.is("id", s) }
func (p *Parser) sym(s string) bool   { return p.is("sym", s) }
func (p *Parser) takeSym(s string) bool {
	if p.sym(s) {
		p.i++
		return true
	}
	return false
}
func (p *Parser) typeName() (string, error) {
	n, e := p.ident()
	if e != nil {
		return "", e
	}
	var b strings.Builder
	b.WriteString(n)
	if p.takeSym("<") {
		b.WriteString("<")
		for {
			t, e := p.typeName()
			if e != nil {
				return "", e
			}
			b.WriteString(t)
			if p.takeSym(">") {
				b.WriteString(">")
				break
			}
			if !p.takeSym(",") {
				return "", p.err("expected ',' or '>' in generic type")
			}
			b.WriteString(",")
		}
	}
	for p.takeSym(".") {
		part, e := p.ident()
		if e != nil {
			return "", e
		}
		b.WriteString(".")
		b.WriteString(part)
	}
	for p.takeSym("[") {
		if !p.takeSym("]") {
			return "", p.err("expected ']' in array type")
		}
		b.WriteString("[]")
	}
	return b.String(), nil
}
func (p *Parser) ident() (string, error) {
	if !p.is("id", "") {
		return "", p.err("expected identifier")
	}
	s := p.cur().S
	p.i++
	return s, nil
}
func (p *Parser) err(s string) error {
	t := p.cur()
	return fmt.Errorf("%d:%d: %s, got %q", t.L, t.C, s, t.S)
}

type Obj struct {
	Class  *Class
	Fields map[string]any
}
type Bound struct {
	O *Obj
	F *Fn
}
type ListVal struct{ X []any }
type MapVal struct{ X map[string]any }
type SetVal struct{ X map[string]any }
type NativeMethod struct {
	O any
	N string
}
type Env struct {
	P *Env
	V map[string]any
}

func (e *Env) get(n string) (any, bool) {
	for q := e; q != nil; q = q.P {
		v, ok := q.V[n]
		if ok {
			return v, true
		}
	}
	return nil, false
}
func (e *Env) set(n string, v any) bool {
	for q := e; q != nil; q = q.P {
		if _, ok := q.V[n]; ok {
			q.V[n] = v
			return true
		}
	}
	return false
}

type Flow struct {
	K string
	V any
}
type VM struct {
	P       *Program
	Out     io.Writer
	Current *Fn
	This    *Obj
	Latch   map[string]bool
	Args    []string
}

func (v *VM) run() error {
	fs := v.P.Fns["main"]
	if len(fs) == 0 {
		return errors.New("no main() function")
	}
	f := fs[0]
	var a []any
	for _, q := range fs {
		if len(q.Params) == 1 {
			f = q
			vals := []any{}
			for _, s := range v.Args {
				vals = append(vals, s)
			}
			a = []any{vals}
			break
		}
	}
	_, e := v.callFn(f, a, nil)
	return e
}
func (v *VM) callFn(f *Fn, args []any, this *Obj) (any, error) {
	if f.Abstract {
		return nil, fmt.Errorf("cannot call abstract function %s", f.Name)
	}
	e := &Env{V: map[string]any{}}
	for i, p := range f.Params {
		if i < len(args) {
			e.V[p.N] = args[i]
		} else {
			e.V[p.N] = nil
		}
	}
	oldF, oldT, oldL := v.Current, v.This, v.Latch
	v.Current, v.This, v.Latch = f, this, map[string]bool{}
	for _, x := range f.Latches {
		v.Latch[x] = true
	}
	if this != nil {
		e.V["this"] = this
	}
	fl, er := v.execList(f.Body, e)
	v.Current, v.This, v.Latch = oldF, oldT, oldL
	if er != nil {
		return nil, er
	}
	if fl != nil && fl.K == "return" {
		return fl.V, nil
	}
	return nil, nil
}
func (v *VM) execList(ss []Stmt, e *Env) (*Flow, error) {
	for _, s := range ss {
		f, er := v.exec(s, e)
		if er != nil {
			return nil, er
		}
		if f != nil {
			return f, nil
		}
	}
	return nil, nil
}
func (v *VM) exec(s Stmt, e *Env) (*Flow, error) {
	switch q := s.(type) {
	case *Block:
		return v.execList(q.S, &Env{P: e, V: map[string]any{}})
	case *Var:
		var x any
		var er error
		if q.X != nil {
			x, er = v.eval(q.X, e)
		}
		if er != nil {
			return nil, er
		}
		e.V[q.N] = x
		return nil, nil
	case *ExprStmt:
		_, er := v.eval(q.X, e)
		return nil, er
	case *If:
		c, er := v.eval(q.C, e)
		if er != nil {
			return nil, er
		}
		if truth(c) {
			return v.exec(q.T, e)
		} else if q.E != nil {
			return v.exec(q.E, e)
		}
	case *While:
		for {
			c, er := v.eval(q.C, e)
			if er != nil {
				return nil, er
			}
			if !truth(c) {
				break
			}
			f, er := v.exec(q.B, e)
			if er != nil {
				return nil, er
			}
			if f != nil {
				if f.K == "break" {
					break
				}
				if f.K == "continue" {
					continue
				}
				return f, nil
			}
		}
	case *For:
		ne := &Env{P: e, V: map[string]any{}}
		if q.Init != nil {
			v.exec(q.Init, ne)
		}
		for {
			if q.C != nil {
				c, er := v.eval(q.C, ne)
				if er != nil {
					return nil, er
				}
				if !truth(c) {
					break
				}
			}
			f, er := v.exec(q.B, ne)
			if er != nil {
				return nil, er
			}
			if f != nil && f.K == "break" {
				break
			}
			if f != nil && f.K == "return" {
				return f, nil
			}
			if q.Post != nil {
				v.eval(q.Post, ne)
			}
		}
	case *Foreach:
		src, er := v.eval(q.X, e)
		if er != nil {
			return nil, er
		}
		var items []any
		switch z := src.(type) {
		case []any:
			items = z
		case *ListVal:
			items = z.X
		case *SetVal:
			for _, x := range z.X {
				items = append(items, x)
			}
		case *MapVal:
			for k := range z.X {
				items = append(items, k)
			}
		case string:
			for _, r := range []rune(z) {
				items = append(items, string(r))
			}
		default:
			return nil, fmt.Errorf("foreach requires an iterable value")
		}
		for _, item := range items {
			ne := &Env{P: e, V: map[string]any{q.N: item}}
			f, er := v.exec(q.B, ne)
			if er != nil {
				return nil, er
			}
			if f != nil {
				if f.K == "break" {
					break
				}
				if f.K == "continue" {
					continue
				}
				return f, nil
			}
		}
	case *Return:
		var x any
		var er error
		if q.X != nil {
			x, er = v.eval(q.X, e)
		}
		return &Flow{"return", x}, er
	case Break:
		return &Flow{"break", nil}, nil
	case Continue:
		return &Flow{"continue", nil}, nil
	}
	return nil, nil
}
func (v *VM) eval(x Expr, e *Env) (any, error) {
	switch q := x.(type) {
	case *Lit:
		return q.V, nil
	case *Name:
		if z, ok := e.get(q.N); ok {
			return z, nil
		}
		// Inside an instance method, an unqualified name falls back to this.<field>.
		// Locals and parameters above intentionally win, matching normal shadowing rules.
		if v.This != nil {
			if z, ok := v.This.Fields[q.N]; ok {
				if fld, owner := findField(v.P, v.This.Class, q.N); fld != nil && !v.canAccess(fld.Mod, owner) {
					return nil, fmt.Errorf("%s field %s.%s is not accessible here", fld.Mod, owner, q.N)
				}
				return z, nil
			}
		}
		if fs := v.P.Fns[q.N]; len(fs) > 0 {
			return fs, nil
		}
		if c := v.P.Classes[q.N]; c != nil {
			return c, nil
		}
		return nil, fmt.Errorf("undefined name %s", q.N)
	case *Array:
		var a []any
		for _, x := range q.X {
			z, er := v.eval(x, e)
			if er != nil {
				return nil, er
			}
			a = append(a, z)
		}
		return a, nil
	case *Unary:
		z, er := v.eval(q.X, e)
		if er != nil {
			return nil, er
		}
		switch q.Op {
		case "!":
			return !truth(z), nil
		case "-":
			return -num(z), nil
		case "+":
			return num(z), nil
		}
	case *Binary:
		if q.Op == "=" || q.Op == "+=" || q.Op == "-=" || q.Op == "*=" || q.Op == "/=" || q.Op == "%=" {
			return v.assign(q, e)
		}
		a, er := v.eval(q.A, e)
		if er != nil {
			return nil, er
		}
		if q.Op == "&&" && !truth(a) {
			return false, nil
		}
		if q.Op == "||" && truth(a) {
			return true, nil
		}
		b, er := v.eval(q.B, e)
		if er != nil {
			return nil, er
		}
		return op(q.Op, a, b)
	case *Member:
		o, er := v.eval(q.O, e)
		if er != nil {
			return nil, er
		}
		switch z := o.(type) {
		case *ListVal:
			if q.N == "count" || q.N == "length" {
				return len(z.X), nil
			}
			return &NativeMethod{z, q.N}, nil
		case *MapVal:
			if q.N == "count" || q.N == "length" {
				return len(z.X), nil
			}
			return &NativeMethod{z, q.N}, nil
		case *SetVal:
			if q.N == "count" || q.N == "length" {
				return len(z.X), nil
			}
			return &NativeMethod{z, q.N}, nil
		}
		if z, ok := o.(*Obj); ok {
			if x, ok := z.Fields[q.N]; ok {
				if fld, owner := findField(v.P, z.Class, q.N); fld != nil && !v.canAccess(fld.Mod, owner) {
					return nil, fmt.Errorf("%s field %s.%s is not accessible here", fld.Mod, owner, q.N)
				}
				return x, nil
			}
			if f := findMethod(v.P, z.Class, q.N); f != nil {
				if !v.canAccess(f.Mod, f.Owner) {
					return nil, fmt.Errorf("%s method %s.%s is not accessible here", f.Mod, f.Owner, q.N)
				}
				return &Bound{z, f}, nil
			}
			return nil, fmt.Errorf("%s has no member %s", z.Class.Name, q.N)
		}
		return nil, fmt.Errorf("member access on non-object")
	case *Index:
		o, er := v.eval(q.O, e)
		if er != nil {
			return nil, er
		}
		i, er := v.eval(q.I, e)
		if er != nil {
			return nil, er
		}
		n := int(num(i))
		switch a := o.(type) {
		case []any:
			if n < 0 || n >= len(a) {
				return nil, fmt.Errorf("array index out of range")
			}
			return a[n], nil
		case *ListVal:
			if n < 0 || n >= len(a.X) {
				return nil, fmt.Errorf("list index out of range")
			}
			return a.X[n], nil
		case *MapVal:
			key := show(i)
			return a.X[key], nil
		case string:
			r := []rune(a)
			if n < 0 || n >= len(r) {
				return nil, fmt.Errorf("string index out of range")
			}
			return string(r[n]), nil
		}
	case *NewObj:
		return v.newObj(q.N, q.Args, e)
	case *Call:
		if m, ok := q.F.(*Member); ok && collectionMutator(m.N) {
			if root, ok := m.O.(*Member); ok {
				if n, ok := root.O.(*Name); ok && n.N == "this" && v.This != nil && !v.Latch["this."+root.N] && !(v.Current != nil && v.Current.Name == v.This.Class.Name) {
					return nil, fmt.Errorf("mutation of this.%s requires latch(this.%s)", root.N, root.N)
				}
			}
		}
		if n, ok := q.F.(*Name); ok && (n.N == "print" || n.N == "println" || n.N == "len" || n.N == "str" || n.N == "int" || n.N == "float") {
			var a []any
			for _, x := range q.Args {
				z, er := v.eval(x, e)
				if er != nil {
					return nil, er
				}
				a = append(a, z)
			}
			return v.builtin(n.N, a)
		}
		var a []any
		for _, x := range q.Args {
			z, er := v.eval(x, e)
			if er != nil {
				return nil, er
			}
			a = append(a, z)
		}
		f, er := v.eval(q.F, e)
		if er != nil {
			return nil, er
		}
		switch z := f.(type) {
		case []*Fn:
			return v.callFn(resolve(z, len(a)), a, nil)
		case *Bound:
			return v.callFn(z.F, a, z.O)
		case *Class:
			return v.newObj(z.Name, q.Args, e)
		case *NativeMethod:
			return v.collectionCall(z, a)
		case string:
			_ = z
		}
		if n, ok := q.F.(*Name); ok {
			return v.builtin(n.N, a)
		}
		return nil, fmt.Errorf("value is not callable")
	}
	return nil, fmt.Errorf("unsupported expression")
}
func (v *VM) assign(q *Binary, e *Env) (any, error) {
	rhs, er := v.eval(q.B, e)
	if er != nil {
		return nil, er
	}
	apply := func(old any) (any, error) {
		if q.Op == "=" {
			return rhs, nil
		}
		return op(strings.TrimSuffix(q.Op, "="), old, rhs)
	}
	switch a := q.A.(type) {
	case *Name:
		old, found := e.get(a.N)
		if !found && v.This != nil {
			if fieldOld, ok := v.This.Fields[a.N]; ok {
				nv, er := apply(fieldOld)
				if er != nil {
					return nil, er
				}
				constructing := v.Current != nil && v.Current.Name == v.This.Class.Name
				if !v.Latch["this."+a.N] && !constructing {
					return nil, fmt.Errorf("mutation of this.%s requires latch(this.%s)", a.N, a.N)
				}
				v.This.Fields[a.N] = nv
				return nv, nil
			}
		}
		nv, er := apply(old)
		if er == nil && !e.set(a.N, nv) {
			e.V[a.N] = nv
		}
		return nv, er
	case *Member:
		o, er := v.eval(a.O, e)
		if er != nil {
			return nil, er
		}
		z, ok := o.(*Obj)
		if !ok {
			return nil, fmt.Errorf("assignment target is not object")
		}
		old := z.Fields[a.N]
		nv, er := apply(old)
		if er != nil {
			return nil, er
		}
		if v.Current != nil && v.This == z {
			constructing := v.Current.Name == z.Class.Name
			allowed := v.Latch["this."+a.N] || constructing
			if !allowed {
				return nil, fmt.Errorf("mutation of this.%s requires latch(this.%s)", a.N, a.N)
			}
		}
		z.Fields[a.N] = nv
		return nv, nil
	case *Index:
		if root, ok := a.O.(*Member); ok {
			if n, ok := root.O.(*Name); ok && n.N == "this" && v.This != nil && !v.Latch["this."+root.N] && !(v.Current != nil && v.Current.Name == v.This.Class.Name) {
				return nil, fmt.Errorf("mutation of this.%s requires latch(this.%s)", root.N, root.N)
			}
		}
		o, er := v.eval(a.O, e)
		if er != nil {
			return nil, er
		}
		ix, er := v.eval(a.I, e)
		if er != nil {
			return nil, er
		}
		nv := rhs
		switch z := o.(type) {
		case *ListVal:
			i := int(num(ix))
			if i < 0 || i >= len(z.X) {
				return nil, fmt.Errorf("list index out of range")
			}
			old := z.X[i]
			nv, er = apply(old)
			if er == nil {
				z.X[i] = nv
			}
			return nv, er
		case []any:
			i := int(num(ix))
			if i < 0 || i >= len(z) {
				return nil, fmt.Errorf("array index out of range")
			}
			old := z[i]
			nv, er = apply(old)
			if er == nil {
				z[i] = nv
			}
			return nv, er
		case *MapVal:
			key := show(ix)
			old := z.X[key]
			nv, er = apply(old)
			if er == nil {
				z.X[key] = nv
			}
			return nv, er
		}
		return nil, fmt.Errorf("indexed assignment requires array/list/map")
	}
	return nil, fmt.Errorf("invalid assignment target")
}
func (v *VM) newObj(n string, args []Expr, e *Env) (any, error) {
	base := n
	if i := strings.Index(base, "<"); i >= 0 {
		base = base[:i]
	}
	switch base {
	case "List":
		return &ListVal{[]any{}}, nil
	case "Map":
		return &MapVal{map[string]any{}}, nil
	case "Set":
		return &SetVal{map[string]any{}}, nil
	}
	c := v.P.Classes[n]
	if c == nil {
		return nil, fmt.Errorf("unknown class %s", n)
	}
	if c.Abstract {
		return nil, fmt.Errorf("cannot instantiate abstract class %s", n)
	}
	o := &Obj{c, map[string]any{}}
	for _, f := range allFields(v.P, c) {
		var z any
		if f.Init != nil {
			z, _ = v.eval(f.Init, e)
		}
		o.Fields[f.N] = z
	}
	if fs := c.Methods[c.Name]; len(fs) > 0 {
		var a []any
		for _, x := range args {
			z, er := v.eval(x, e)
			if er != nil {
				return nil, er
			}
			a = append(a, z)
		}
		if _, er := v.callFn(resolve(fs, len(a)), a, o); er != nil {
			return nil, er
		}
	}
	return o, nil
}
func collectionMutator(n string) bool {
	return n == "add" || n == "remove" || n == "removeAt" || n == "clear" || n == "set"
}

func (v *VM) collectionCall(m *NativeMethod, a []any) (any, error) {
	switch z := m.O.(type) {
	case *ListVal:
		switch m.N {
		case "add":
			if len(a) != 1 {
				return nil, fmt.Errorf("List.add expects 1 argument")
			}
			z.X = append(z.X, a[0])
			return nil, nil
		case "removeAt":
			if len(a) != 1 {
				return nil, fmt.Errorf("List.removeAt expects 1 argument")
			}
			i := int(num(a[0]))
			if i < 0 || i >= len(z.X) {
				return nil, fmt.Errorf("list index out of range")
			}
			old := z.X[i]
			z.X = append(z.X[:i], z.X[i+1:]...)
			return old, nil
		case "contains":
			for _, x := range z.X {
				if show(x) == show(a[0]) {
					return true, nil
				}
			}
			return false, nil
		case "clear":
			z.X = nil
			return nil, nil
		}
	case *MapVal:
		switch m.N {
		case "set":
			if len(a) != 2 {
				return nil, fmt.Errorf("Map.set expects 2 arguments")
			}
			z.X[show(a[0])] = a[1]
			return nil, nil
		case "get":
			if len(a) != 1 {
				return nil, fmt.Errorf("Map.get expects 1 argument")
			}
			return z.X[show(a[0])], nil
		case "containsKey":
			_, ok := z.X[show(a[0])]
			return ok, nil
		case "remove":
			delete(z.X, show(a[0]))
			return nil, nil
		case "clear":
			z.X = map[string]any{}
			return nil, nil
		}
	case *SetVal:
		switch m.N {
		case "add":
			if len(a) != 1 {
				return nil, fmt.Errorf("Set.add expects 1 argument")
			}
			z.X[show(a[0])] = a[0]
			return nil, nil
		case "contains":
			_, ok := z.X[show(a[0])]
			return ok, nil
		case "remove":
			delete(z.X, show(a[0]))
			return nil, nil
		case "clear":
			z.X = map[string]any{}
			return nil, nil
		}
	}
	return nil, fmt.Errorf("unknown collection method %s", m.N)
}
func (v *VM) builtin(n string, a []any) (any, error) {
	switch n {
	case "print":
		for _, x := range a {
			fmt.Fprint(v.Out, show(x))
		}
		return nil, nil
	case "println":
		for _, x := range a {
			fmt.Fprint(v.Out, show(x))
		}
		fmt.Fprintln(v.Out)
		return nil, nil
	case "len":
		if len(a) != 1 {
			return nil, fmt.Errorf("len expects 1 argument")
		}
		switch z := a[0].(type) {
		case []any:
			return len(z), nil
		case string:
			return len([]rune(z)), nil
		case *ListVal:
			return len(z.X), nil
		case *MapVal:
			return len(z.X), nil
		case *SetVal:
			return len(z.X), nil
		}
	case "str":
		if len(a) == 1 {
			return show(a[0]), nil
		}
	case "int":
		if len(a) == 1 {
			return int(num(a[0])), nil
		}
	case "float":
		if len(a) == 1 {
			return num(a[0]), nil
		}
	}
	return nil, fmt.Errorf("unknown function %s", n)
}
func resolve(fs []*Fn, n int) *Fn {
	for _, f := range fs {
		if len(f.Params) == n {
			return f
		}
	}
	return fs[0]
}
func findMethod(p *Program, c *Class, n string) *Fn {
	for q := c; q != nil; {
		if fs := q.Methods[n]; len(fs) > 0 {
			return fs[0]
		}
		if q.Base == "" {
			break
		}
		q = p.Classes[q.Base]
	}
	return nil
}

func findField(p *Program, c *Class, n string) (*Field, string) {
	for q := c; q != nil; {
		for i := range q.Fields {
			if q.Fields[i].N == n {
				return &q.Fields[i], q.Name
			}
		}
		if q.Base == "" {
			break
		}
		q = p.Classes[q.Base]
	}
	return nil, ""
}
func (v *VM) canAccess(mod, owner string) bool {
	if mod == "public" || mod == "internal" {
		return true
	}
	cur := ""
	if v.Current != nil {
		cur = v.Current.Owner
	}
	if cur == owner {
		return true
	}
	if mod == "protected" && cur != "" {
		for c := v.P.Classes[cur]; c != nil && c.Base != ""; {
			if c.Base == owner {
				return true
			}
			c = v.P.Classes[c.Base]
		}
	}
	return false
}

func allFields(p *Program, c *Class) []Field {
	var a []Field
	if c.Base != "" && p.Classes[c.Base] != nil {
		a = append(a, allFields(p, p.Classes[c.Base])...)
	}
	return append(a, c.Fields...)
}
func truth(x any) bool {
	switch z := x.(type) {
	case bool:
		return z
	case nil:
		return false
	case int:
		return z != 0
	case float64:
		return z != 0
	case string:
		return z != ""
	}
	return true
}
func num(x any) float64 {
	switch z := x.(type) {
	case int:
		return float64(z)
	case float64:
		return z
	case bool:
		if z {
			return 1
		}
	}
	return 0
}
func show(x any) string {
	switch z := x.(type) {
	case nil:
		return "null"
	case float64:
		if z == float64(int(z)) {
			return strconv.Itoa(int(z))
		}
		return strconv.FormatFloat(z, 'g', -1, 64)
	case *Obj:
		return z.Class.Name
	case *ListVal:
		return fmt.Sprint(z.X)
	case *MapVal:
		return fmt.Sprint(z.X)
	case *SetVal:
		return fmt.Sprint(z.X)
	default:
		return fmt.Sprint(z)
	}
}
func op(o string, a, b any) (any, error) {
	if o == "+" {
		if _, ok := a.(string); ok {
			return show(a) + show(b), nil
		}
		if _, ok := b.(string); ok {
			return show(a) + show(b), nil
		}
	}
	switch o {
	case "+":
		return num(a) + num(b), nil
	case "-":
		return num(a) - num(b), nil
	case "*":
		return num(a) * num(b), nil
	case "/":
		if num(b) == 0 {
			return nil, errors.New("division by zero")
		}
		return num(a) / num(b), nil
	case "%":
		return int(num(a)) % int(num(b)), nil
	case "==":
		return show(a) == show(b), nil
	case "!=":
		return show(a) != show(b), nil
	case "<":
		return num(a) < num(b), nil
	case "<=":
		return num(a) <= num(b), nil
	case ">":
		return num(a) > num(b), nil
	case ">=":
		return num(a) >= num(b), nil
	case "&&":
		return truth(a) && truth(b), nil
	case "||":
		return truth(a) || truth(b), nil
	}
	return nil, fmt.Errorf("unsupported operator %s", o)
}

// Alpha 2.5 semantic validation. This pass is deliberately side-effect free: it walks
// declarations and executable AST before the VM is allowed to run.
type Sem struct {
	P    *Program
	Errs []string
}
type SemEnv struct {
	P    *SemEnv
	V    map[string]string
	This *Class
	Fn   *Fn
	Loop int
}

func (e *SemEnv) get(n string) (string, bool) {
	for q := e; q != nil; q = q.P {
		if t, ok := q.V[n]; ok {
			return t, true
		}
	}
	return "", false
}
func semAssignable(want, got string) bool {
	if want == "" || got == "" || want == got {
		return true
	}
	if want == "float" && got == "int" {
		return true
	}
	return false
}
func (s *Sem) errf(f string, a ...any) { s.Errs = append(s.Errs, fmt.Sprintf(f, a...)) }
func (s *Sem) field(c *Class, n string) (*Field, *Class) {
	for q := c; q != nil; {
		for i := range q.Fields {
			if q.Fields[i].N == n {
				return &q.Fields[i], q
			}
		}
		if q.Base == "" {
			break
		}
		q = s.P.Classes[q.Base]
	}
	return nil, nil
}
func (s *Sem) method(c *Class, n string) *Fn {
	for q := c; q != nil; {
		if xs := q.Methods[n]; len(xs) > 0 {
			return xs[0]
		}
		if q.Base == "" {
			break
		}
		q = s.P.Classes[q.Base]
	}
	return nil
}
func (s *Sem) expr(x Expr, e *SemEnv) string {
	switch q := x.(type) {
	case *Lit:
		switch q.V.(type) {
		case int, int64:
			return "int"
		case float64:
			return "float"
		case bool:
			return "bool"
		case string:
			return "string"
		}
	case *Name:
		if t, ok := e.get(q.N); ok {
			return t
		}
		if e.This != nil {
			if f, _ := s.field(e.This, q.N); f != nil {
				return f.Type
			}
			if m := s.method(e.This, q.N); m != nil {
				return "fn:" + m.Ret
			}
		}
		if c := s.P.Classes[q.N]; c != nil {
			return "type:" + c.Name
		}
		if fs := s.P.Fns[q.N]; len(fs) > 0 {
			return "fn:" + fs[0].Ret
		}
		if q.N == "true" || q.N == "false" {
			return "bool"
		}
		s.errf("undefined name %s", q.N)
	case *NewObj:
		c := s.P.Classes[q.N]
		if c == nil {
			s.errf("unknown type %s", q.N)
			return q.N
		}
		for _, a := range q.Args {
			s.expr(a, e)
		}
		return q.N
	case *Array:
		for _, a := range q.X {
			s.expr(a, e)
		}
		return "array"
	case *Unary:
		return s.expr(q.X, e)
	case *Binary:
		a := s.expr(q.A, e)
		b := s.expr(q.B, e)
		if strings.Contains(" = += -= *= /= %= ", " "+q.Op+" ") {
			if !semAssignable(a, b) {
				s.errf("cannot assign %s to %s", b, a)
			}
			return a
		}
		if strings.Contains(" == != < <= > >= && || ", " "+q.Op+" ") {
			return "bool"
		}
		if a == "string" && q.Op == "+" {
			return "string"
		}
		if a == "float" || b == "float" {
			return "float"
		}
		return a
	case *Member:
		ot := s.expr(q.O, e)
		if strings.HasPrefix(ot, "type:") {
			c := s.P.Classes[strings.TrimPrefix(ot, "type:")]
			if c != nil {
				if f, _ := s.field(c, q.N); f != nil {
					if !f.Static {
						s.errf("instance member %s.%s requires an object", c.Name, q.N)
					}
					return f.Type
				}
				if m := s.method(c, q.N); m != nil {
					return "fn:" + m.Ret
				}
			}
		}
		c := s.P.Classes[ot]
		if c == nil {
			if ot == "array" || strings.HasPrefix(ot, "List") || strings.HasPrefix(ot, "Map") || strings.HasPrefix(ot, "Set") {
				return "native"
			}
			s.errf("member access on non-object type %s", ot)
			return ""
		}
		if f, owner := s.field(c, q.N); f != nil {
			if f.Mod == "private" && e.This != owner {
				s.errf("private field %s.%s is not accessible here", owner.Name, q.N)
			}
			return f.Type
		}
		if m := s.method(c, q.N); m != nil {
			if m.Mod == "private" && (e.This == nil || e.This.Name != m.Owner) {
				s.errf("private method %s.%s is not accessible here", m.Owner, q.N)
			}
			return "fn:" + m.Ret
		}
		s.errf("%s has no member %s", c.Name, q.N)
	case *Index:
		s.expr(q.I, e)
		return s.expr(q.O, e)
	case *Call:
		if n, ok := q.F.(*Name); ok && (n.N == "print" || n.N == "println" || n.N == "len" || n.N == "str" || n.N == "int" || n.N == "float") {
			for _, a := range q.Args {
				s.expr(a, e)
			}
			if n.N == "len" || n.N == "int" {
				return "int"
			}
			if n.N == "float" {
				return "float"
			}
			if n.N == "str" {
				return "string"
			}
			return "void"
		}
		ft := s.expr(q.F, e)
		var f *Fn
		switch z := q.F.(type) {
		case *Name:
			if xs := s.P.Fns[z.N]; len(xs) > 0 {
				f = xs[0]
			}
			if e.This != nil && f == nil {
				f = s.method(e.This, z.N)
			}
		case *Member:
			ot := s.expr(z.O, e)
			f = s.method(s.P.Classes[ot], z.N)
		}
		if f != nil {
			if len(q.Args) != len(f.Params) {
				s.errf("%s expects %d argument(s), got %d", f.Name, len(f.Params), len(q.Args))
			}
			for i, a := range q.Args {
				at := s.expr(a, e)
				if i < len(f.Params) && !semAssignable(f.Params[i].Type, at) {
					s.errf("argument %d of %s expects %s, got %s", i+1, f.Name, f.Params[i].Type, at)
				}
			}
			return f.Ret
		}
		for _, a := range q.Args {
			s.expr(a, e)
		}
		if strings.HasPrefix(ft, "fn:") {
			return strings.TrimPrefix(ft, "fn:")
		}
		return ""
	}
	return ""
}
func (s *Sem) stmt(x Stmt, e *SemEnv) {
	switch q := x.(type) {
	case *Block:
		n := &SemEnv{P: e, V: map[string]string{}, This: e.This, Fn: e.Fn, Loop: e.Loop}
		for _, z := range q.S {
			s.stmt(z, n)
		}
	case *Var:
		if !isKnownType(s.P, q.Type) {
			s.errf("variable %s uses unknown type %s", q.N, q.Type)
		}
		if _, ok := e.V[q.N]; ok {
			s.errf("duplicate local %s", q.N)
		}
		if q.X != nil {
			t := s.expr(q.X, e)
			if !semAssignable(q.Type, t) {
				s.errf("cannot initialize %s with %s", q.Type, t)
			}
		}
		e.V[q.N] = q.Type
	case *ExprStmt:
		s.expr(q.X, e)
	case *If:
		s.expr(q.C, e)
		s.stmt(q.T, e)
		if q.E != nil {
			s.stmt(q.E, e)
		}
	case *While:
		s.expr(q.C, e)
		n := *e
		n.Loop++
		s.stmt(q.B, &n)
	case *For:
		n := &SemEnv{P: e, V: map[string]string{}, This: e.This, Fn: e.Fn, Loop: e.Loop + 1}
		if q.Init != nil {
			s.stmt(q.Init, n)
		}
		if q.C != nil {
			s.expr(q.C, n)
		}
		if q.Post != nil {
			s.expr(q.Post, n)
		}
		s.stmt(q.B, n)
	case *Foreach:
		s.expr(q.X, e)
		n := &SemEnv{P: e, V: map[string]string{q.N: q.Type}, This: e.This, Fn: e.Fn, Loop: e.Loop + 1}
		s.stmt(q.B, n)
	case *Return:
		t := "void"
		if q.X != nil {
			t = s.expr(q.X, e)
		}
		if e.Fn != nil && !semAssignable(e.Fn.Ret, t) {
			s.errf("function %s returns %s but expression is %s", e.Fn.Name, e.Fn.Ret, t)
		}
	case Break:
		if e.Loop == 0 {
			s.errf("break is only valid inside a loop")
		}
	case Continue:
		if e.Loop == 0 {
			s.errf("continue is only valid inside a loop")
		}
	}
}
func isKnownType(p *Program, t string) bool {
	if t == "int" || t == "float" || t == "bool" || t == "string" || t == "char" || t == "void" || strings.HasSuffix(t, "[]") || strings.HasPrefix(t, "List<") || strings.HasPrefix(t, "Map<") || strings.HasPrefix(t, "Set<") {
		return true
	}
	return p.Classes[t] != nil || p.Interfaces[t] != nil || p.Enums[t] != nil
}
func validate(p *Program) error {
	s := &Sem{P: p}
	for _, c := range p.Classes {
		if c.Base != "" && p.Classes[c.Base] == nil {
			s.errf("class %s extends unknown class %s", c.Name, c.Base)
		}
		seen := map[string]bool{}
		for _, f := range c.Fields {
			if seen[f.N] {
				s.errf("duplicate field %s.%s", c.Name, f.N)
			}
			seen[f.N] = true
			if f.Init != nil {
				env := &SemEnv{V: map[string]string{}, This: c}
				got := s.expr(f.Init, env)
				if !semAssignable(f.Type, got) {
					s.errf("field %s.%s expects %s, got %s", c.Name, f.N, f.Type, got)
				}
			}
			if !isKnownType(p, f.Type) {
				s.errf("field %s.%s uses unknown type %s", c.Name, f.N, f.Type)
			}
		}
		for _, xs := range c.Methods {
			for _, f := range xs {
				e := &SemEnv{V: map[string]string{}, This: c, Fn: f}
				for _, a := range f.Params {
					e.V[a.N] = a.Type
				}
				for _, z := range f.Body {
					s.stmt(z, e)
				}
			}
		}
	}
	for _, xs := range p.Fns {
		for _, f := range xs {
			e := &SemEnv{V: map[string]string{}, Fn: f}
			for _, a := range f.Params {
				e.V[a.N] = a.Type
			}
			for _, z := range f.Body {
				s.stmt(z, e)
			}
		}
	}
	if len(p.Fns["main"]) == 0 {
		s.errf("no main() function")
	}
	if len(s.Errs) > 0 {
		return fmt.Errorf("semantic validation failed:\n - %s", strings.Join(s.Errs, "\n - "))
	}
	return nil
}

func load(path string, seen map[string]bool) (string, error) {
	abs, _ := filepath.Abs(path)
	if seen[abs] {
		return "", nil
	}
	seen[abs] = true
	b, e := os.ReadFile(abs)
	if e != nil {
		return "", e
	}
	src := string(b)
	dir := filepath.Dir(abs)
	re := strings.Split(src, "\n")
	var pre strings.Builder
	for _, ln := range re {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "import ") {
			q := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "import "), ";"))
			q = strings.Trim(q, "\"")
			if !strings.HasSuffix(q, ".lt") {
				q = strings.ReplaceAll(q, ".", string(os.PathSeparator)) + ".lt"
			}
			importPath := filepath.Join(dir, q)
			if _, statErr := os.Stat(importPath); statErr != nil {
				// Project libraries: walk upward looking for lib/<module>.
				for d := dir; ; d = filepath.Dir(d) {
					cand := filepath.Join(d, "lib", q)
					if _, er := os.Stat(cand); er == nil {
						importPath = cand
						break
					}
					next := filepath.Dir(d)
					if next == d {
						break
					}
				}
				// SDK libraries live beside bin/ in <sdk>/lib.
				if _, er := os.Stat(importPath); er != nil {
					if exe, exErr := os.Executable(); exErr == nil {
						cand := filepath.Join(filepath.Dir(filepath.Dir(exe)), "lib", q)
						if _, er2 := os.Stat(cand); er2 == nil {
							importPath = cand
						}
					}
				}
				// Optional extra library roots, separated using the platform path-list separator.
				if _, er := os.Stat(importPath); er != nil {
					for _, root := range filepath.SplitList(os.Getenv("LATCH_PATH")) {
						cand := filepath.Join(root, q)
						if _, er2 := os.Stat(cand); er2 == nil {
							importPath = cand
							break
						}
					}
				}
			}
			s, e := load(importPath, seen)
			if e != nil {
				return "", fmt.Errorf("cannot import %s: %w", q, e)
			}
			pre.WriteString(s)
			pre.WriteByte('\n')
		} else {
			pre.WriteString(ln)
			pre.WriteByte('\n')
		}
	}
	return pre.String(), nil
}
func main() {
	if len(os.Args) < 3 {
		fmt.Println("Latch 1.0\nusage: latch <run|check> <file.lt>")
		return
	}
	src, e := load(os.Args[2], map[string]bool{})
	if e != nil {
		fmt.Fprintln(os.Stderr, "Latch error:", e)
		os.Exit(1)
	}
	ts, e := lex(src)
	if e != nil {
		fmt.Fprintln(os.Stderr, "Latch error:", e)
		os.Exit(1)
	}
	p, e := parse(ts)
	if e != nil {
		fmt.Fprintln(os.Stderr, "Latch error:", e)
		os.Exit(1)
	}
	if e = validate(p); e != nil {
		fmt.Fprintln(os.Stderr, "Latch error:", e)
		os.Exit(1)
	}
	if os.Args[1] == "check" {
		// Alpha 2.4 preflight: execute against a discarded output stream so name/member/
		// latch/runtime validation completes without allowing user-visible program output.
		v := &VM{P: p, Out: io.Discard, Args: os.Args[3:]}
		if e = v.run(); e != nil {
			fmt.Fprintln(os.Stderr, "Latch error:", e)
			os.Exit(1)
		}
		fmt.Println("Latch check passed.")
		return
	}
	if os.Args[1] != "run" {
		fmt.Fprintln(os.Stderr, "Latch error: unknown command")
		os.Exit(1)
	}
	// Never produce program output until the program passes preflight validation.
	preflight := &VM{P: p, Out: io.Discard, Args: os.Args[3:]}
	if e = preflight.run(); e != nil {
		fmt.Fprintln(os.Stderr, "Latch error:", e)
		os.Exit(1)
	}
	v := &VM{P: p, Out: os.Stdout, Args: os.Args[3:]}
	if e = v.run(); e != nil {
		fmt.Fprintln(os.Stderr, "Latch error:", e)
		os.Exit(1)
	}
}
