// Inferno utils/6l/span.c
// https://bitbucket.org/inferno-os/inferno-os/src/default/utils/6l/span.c
//
//	Copyright © 1994-1999 Lucent Technologies Inc.  All rights reserved.
//	Portions Copyright © 1995-1997 C H Forsyth (forsyth@terzarima.net)
//	Portions Copyright © 1997-1999 Vita Nuova Limited
//	Portions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com)
//	Portions Copyright © 2004,2006 Bruce Ellis
//	Portions Copyright © 2005-2007 C H Forsyth (forsyth@terzarima.net)
//	Revisions Copyright © 2000-2007 Lucent Technologies Inc. and others
//	Portions Copyright © 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package x86

import (
	"encoding/binary"
	"fmt"
	"github.com/dave/golib/src/cmd/internal/obj"
	"github.com/dave/golib/src/cmd/internal/objabi"
	"github.com/dave/golib/src/cmd/internal/sys"
	"log"
	"strings"
)

// Instruction layout.

// Loop alignment constants:
// want to align loop entry to loopAlign-byte boundary,
// and willing to insert at most maxLoopPad bytes of NOP to do so.
// We define a loop entry as the target of a backward jump.
//
// gcc uses maxLoopPad = 10 for its 'generic x86-64' config,
// and it aligns all jump targets, not just backward jump targets.
//
// As of 6/1/2012, the effect of setting maxLoopPad = 10 here
// is very slight but negative, so the alignment is disabled by
// setting MaxLoopPad = 0. The code is here for reference and
// for future experiments.
//
const (
	loopAlign  = 16
	maxLoopPad = 0
)

// Bit flags that are used to express jump target properties.
const (
	// branchBackwards marks targets that are located behind.
	// Used to express jumps to loop headers.
	branchBackwards = (1 << iota)
	// branchShort marks branches those target is close,
	// with offset is in -128..127 range.
	branchShort
	// branchLoopHead marks loop entry.
	// Used to insert padding for misaligned loops.
	branchLoopHead
)

// opBytes holds optab encoding bytes.
// Each ytab reserves fixed amount of bytes in this array.
//
// The size should be the minimal number of bytes that
// are enough to hold biggest optab op lines.
type opBytes [31]uint8

type Optab struct {
	as     obj.As
	ytab   []ytab
	prefix uint8
	op     opBytes
}

type Movtab struct {
	as   obj.As
	ft   uint8
	f3t  uint8
	tt   uint8
	code uint8
	op   [4]uint8
}

const (
	Yxxx = iota
	Ynone
	Yi0 // $0
	Yi1 // $1
	Yu2 // $x, x fits in uint2
	Yi8 // $x, x fits in int8
	Yu8 // $x, x fits in uint8
	Yu7 // $x, x in 0..127 (fits in both int8 and uint8)
	Ys32
	Yi32
	Yi64
	Yiauto
	Yal
	Ycl
	Yax
	Ycx
	Yrb
	Yrl
	Yrl32 // Yrl on 32-bit system
	Yrf
	Yf0
	Yrx
	Ymb
	Yml
	Ym
	Ybr
	Ycs
	Yss
	Yds
	Yes
	Yfs
	Ygs
	Ygdtr
	Yidtr
	Yldtr
	Ymsw
	Ytask
	Ycr0
	Ycr1
	Ycr2
	Ycr3
	Ycr4
	Ycr5
	Ycr6
	Ycr7
	Ycr8
	Ydr0
	Ydr1
	Ydr2
	Ydr3
	Ydr4
	Ydr5
	Ydr6
	Ydr7
	Ytr0
	Ytr1
	Ytr2
	Ytr3
	Ytr4
	Ytr5
	Ytr6
	Ytr7
	Ymr
	Ymm
	Yxr0          // X0 only. "<XMM0>" notation in Intel manual.
	YxrEvexMulti4 // [ X<n> - X<n+3> ]; multisource YxrEvex
	Yxr           // X0..X15
	YxrEvex       // X0..X31
	Yxm
	YxmEvex       // YxrEvex+Ym
	Yxvm          // VSIB vector array; vm32x/vm64x
	YxvmEvex      // Yxvm which permits High-16 X register as index.
	YyrEvexMulti4 // [ Y<n> - Y<n+3> ]; multisource YyrEvex
	Yyr           // Y0..Y15
	YyrEvex       // Y0..Y31
	Yym
	YymEvex   // YyrEvex+Ym
	Yyvm      // VSIB vector array; vm32y/vm64y
	YyvmEvex  // Yyvm which permits High-16 Y register as index.
	YzrMulti4 // [ Z<n> - Z<n+3> ]; multisource YzrEvex
	Yzr       // Z0..Z31
	Yzm       // Yzr+Ym
	Yzvm      // VSIB vector array; vm32z/vm64z
	Yk0       // K0
	Yknot0    // K1..K7; write mask
	Yk        // K0..K7; used for KOP
	Ykm       // Yk+Ym; used for KOP
	Ytls
	Ytextsize
	Yindir
	Ymax
)

const (
	Zxxx = iota
	Zlit
	Zlitm_r
	Zlitr_m
	Zlit_m_r
	Z_rp
	Zbr
	Zcall
	Zcallcon
	Zcallduff
	Zcallind
	Zcallindreg
	Zib_
	Zib_rp
	Zibo_m
	Zibo_m_xm
	Zil_
	Zil_rp
	Ziq_rp
	Zilo_m
	Zjmp
	Zjmpcon
	Zloop
	Zo_iw
	Zm_o
	Zm_r
	Z_m_r
	Zm2_r
	Zm_r_xm
	Zm_r_i_xm
	Zm_r_xm_nr
	Zr_m_xm_nr
	Zibm_r // mmx1,mmx2/mem64,imm8
	Zibr_m
	Zmb_r
	Zaut_r
	Zo_m
	Zo_m64
	Zpseudo
	Zr_m
	Zr_m_xm
	Zrp_
	Z_ib
	Z_il
	Zm_ibo
	Zm_ilo
	Zib_rr
	Zil_rr
	Zbyte

	Zvex_rm_v_r
	Zvex_rm_v_ro
	Zvex_r_v_rm
	Zvex_i_rm_vo
	Zvex_v_rm_r
	Zvex_i_rm_r
	Zvex_i_r_v
	Zvex_i_rm_v_r
	Zvex
	Zvex_rm_r_vo
	Zvex_i_r_rm
	Zvex_hr_rm_v_r

	Zevex_first
	Zevex_i_r_k_rm
	Zevex_i_r_rm
	Zevex_i_rm_k_r
	Zevex_i_rm_k_vo
	Zevex_i_rm_r
	Zevex_i_rm_v_k_r
	Zevex_i_rm_v_r
	Zevex_i_rm_vo
	Zevex_k_rmo
	Zevex_r_k_rm
	Zevex_r_v_k_rm
	Zevex_r_v_rm
	Zevex_rm_k_r
	Zevex_rm_v_k_r
	Zevex_rm_v_r
	Zevex_last

	Zmax
)

const (
	Px   = 0
	Px1  = 1    // symbolic; exact value doesn't matter
	P32  = 0x32 // 32-bit only
	Pe   = 0x66 // operand escape
	Pm   = 0x0f // 2byte opcode escape
	Pq   = 0xff // both escapes: 66 0f
	Pb   = 0xfe // byte operands
	Pf2  = 0xf2 // xmm escape 1: f2 0f
	Pf3  = 0xf3 // xmm escape 2: f3 0f
	Pef3 = 0xf5 // xmm escape 2 with 16-bit prefix: 66 f3 0f
	Pq3  = 0x67 // xmm escape 3: 66 48 0f
	Pq4  = 0x68 // xmm escape 4: 66 0F 38
	Pq4w = 0x69 // Pq4 with Rex.w 66 0F 38
	Pq5  = 0x6a // xmm escape 5: F3 0F 38
	Pq5w = 0x6b // Pq5 with Rex.w F3 0F 38
	Pfw  = 0xf4 // Pf3 with Rex.w: f3 48 0f
	Pw   = 0x48 // Rex.w
	Pw8  = 0x90 // symbolic; exact value doesn't matter
	Py   = 0x80 // defaults to 64-bit mode
	Py1  = 0x81 // symbolic; exact value doesn't matter
	Py3  = 0x83 // symbolic; exact value doesn't matter
	Pavx = 0x84 // symbolic: exact value doesn't matter

	RxrEvex = 1 << 4 // AVX512 extension to REX.R/VEX.R
	Rxw     = 1 << 3 // =1, 64-bit operand size
	Rxr     = 1 << 2 // extend modrm reg
	Rxx     = 1 << 1 // extend sib index
	Rxb     = 1 << 0 // extend modrm r/m, sib base, or opcode reg
)

const (
	// Encoding for VEX prefix in tables.
	// The P, L, and W fields are chosen to match
	// their eventual locations in the VEX prefix bytes.

	// Encoding for VEX prefix in tables.
	// The P, L, and W fields are chosen to match
	// their eventual locations in the VEX prefix bytes.

	// Using spare bit to make leading [E]VEX encoding byte different from
	// 0x0f even if all other VEX fields are 0.
	avxEscape = 1 << 6

	// P field - 2 bits
	vex66 = 1 << 0
	vexF3 = 2 << 0
	vexF2 = 3 << 0
	// L field - 1 bit
	vexLZ  = 0 << 2
	vexLIG = 0 << 2
	vex128 = 0 << 2
	vex256 = 1 << 2
	// W field - 1 bit
	vexWIG = 0 << 7
	vexW0  = 0 << 7
	vexW1  = 1 << 7
	// M field - 5 bits, but mostly reserved; we can store up to 3
	vex0F   = 1 << 3
	vex0F38 = 2 << 3
	vex0F3A = 3 << 3
)

// useAbs reports whether s describes a symbol that must avoid pc-relative addressing.
// This happens on systems like Solaris that call .so functions instead of system calls.
// It does not seem to be necessary for any other systems. This is probably working
// around a Solaris-specific bug that should be fixed differently, but we don't know
// what that bug is. And this does fix it.
func useAbs(ctxt *obj.Link, s *obj.LSym) bool {
	if ctxt.Headtype == objabi.Hsolaris {
		// All the Solaris dynamic imports from libc.so begin with "libc_".
		return strings.HasPrefix(s.Name, "libc_")
	}
	return ctxt.Arch.Family == sys.I386 && !ctxt.Flag_shared
}

// Native Client rejects the repeated 0x66 prefix.
// {0x66, 0x66, 0x0F, 0x1F, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00},
func (pstate *PackageState) fillnop(p []byte, n int) {
	var m int

	for n > 0 {
		m = n
		if m > len(pstate.nop) {
			m = len(pstate.nop)
		}
		copy(p[:m], pstate.nop[m-1][:m])
		p = p[m:]
		n -= m
	}
}

func (pstate *PackageState) naclpad(ctxt *obj.Link, s *obj.LSym, c int32, pad int32) int32 {
	s.Grow(int64(c) + int64(pad))
	pstate.fillnop(s.P[c:], int(pad))
	return c + pad
}

func spadjop(ctxt *obj.Link, l, q obj.As) obj.As {
	if ctxt.Arch.Family != sys.AMD64 || ctxt.Arch.PtrSize == 4 {
		return l
	}
	return q
}

func (pstate *PackageState) span6(ctxt *obj.Link, s *obj.LSym, newprog obj.ProgAlloc) {
	if s.P != nil {
		return
	}

	if pstate.ycover[0] == 0 {
		ctxt.Diag("x86 tables not initialized, call x86.instinit first")
	}

	var ab AsmBuf

	for p := s.Func.Text; p != nil; p = p.Link {
		if p.To.Type == obj.TYPE_BRANCH {
			if p.Pcond == nil {
				p.Pcond = p
			}
		}
		if p.As == AADJSP {
			p.To.Type = obj.TYPE_REG
			p.To.Reg = REG_SP
			v := int32(-p.From.Offset)
			p.From.Offset = int64(v)
			p.As = spadjop(ctxt, AADDL, AADDQ)
			if v < 0 {
				p.As = spadjop(ctxt, ASUBL, ASUBQ)
				v = -v
				p.From.Offset = int64(v)
			}

			if v == 0 {
				p.As = obj.ANOP
			}
		}
	}

	var q *obj.Prog
	var count int64 // rough count of number of instructions
	for p := s.Func.Text; p != nil; p = p.Link {
		count++
		p.Back = branchShort // use short branches first time through
		q = p.Pcond
		if q != nil && (q.Back&branchShort != 0) {
			p.Back |= branchBackwards
			q.Back |= branchLoopHead
		}

		if p.As == AADJSP {
			p.To.Type = obj.TYPE_REG
			p.To.Reg = REG_SP
			v := int32(-p.From.Offset)
			p.From.Offset = int64(v)
			p.As = spadjop(ctxt, AADDL, AADDQ)
			if v < 0 {
				p.As = spadjop(ctxt, ASUBL, ASUBQ)
				v = -v
				p.From.Offset = int64(v)
			}

			if v == 0 {
				p.As = obj.ANOP
			}
		}
	}
	s.GrowCap(count * 5) // preallocate roughly 5 bytes per instruction

	n := 0
	var c int32
	errors := ctxt.Errors
	for {
		// This loop continues while there are reasons to re-assemble
		// whole block, like the presence of long forward jumps.
		reAssemble := false
		for i := range s.R {
			s.R[i] = obj.Reloc{}
		}
		s.R = s.R[:0]
		s.P = s.P[:0]
		c = 0
		for p := s.Func.Text; p != nil; p = p.Link {
			if ctxt.Headtype == objabi.Hnacl && p.Isize > 0 {
				// pad everything to avoid crossing 32-byte boundary
				if c>>5 != (c+int32(p.Isize)-1)>>5 {
					c = pstate.naclpad(ctxt, s, c, -c&31)
				}

				// pad call deferreturn to start at 32-byte boundary
				// so that subtracting 5 in jmpdefer will jump back
				// to that boundary and rerun the call.
				if p.As == obj.ACALL && p.To.Sym == pstate.deferreturn {
					c = pstate.naclpad(ctxt, s, c, -c&31)
				}

				// pad call to end at 32-byte boundary
				if p.As == obj.ACALL {
					c = pstate.naclpad(ctxt, s, c, -(c+int32(p.Isize))&31)
				}

				// the linker treats REP and STOSQ as different instructions
				// but in fact the REP is a prefix on the STOSQ.
				// make sure REP has room for 2 more bytes, so that
				// padding will not be inserted before the next instruction.
				if (p.As == AREP || p.As == AREPN) && c>>5 != (c+3-1)>>5 {
					c = pstate.naclpad(ctxt, s, c, -c&31)
				}

				// same for LOCK.
				// various instructions follow; the longest is 4 bytes.
				// give ourselves 8 bytes so as to avoid surprises.
				if p.As == ALOCK && c>>5 != (c+8-1)>>5 {
					c = pstate.naclpad(ctxt, s, c, -c&31)
				}
			}

			if (p.Back&branchLoopHead != 0) && c&(loopAlign-1) != 0 {
				// pad with NOPs
				v := -c & (loopAlign - 1)

				if v <= maxLoopPad {
					s.Grow(int64(c) + int64(v))
					pstate.fillnop(s.P[c:], int(v))
					c += v
				}
			}

			p.Pc = int64(c)

			// process forward jumps to p
			for q = p.Rel; q != nil; q = q.Forwd {
				v := int32(p.Pc - (q.Pc + int64(q.Isize)))
				if q.Back&branchShort != 0 {
					if v > 127 {
						reAssemble = true
						q.Back ^= branchShort
					}

					if q.As == AJCXZL || q.As == AXBEGIN {
						s.P[q.Pc+2] = byte(v)
					} else {
						s.P[q.Pc+1] = byte(v)
					}
				} else {
					binary.LittleEndian.PutUint32(s.P[q.Pc+int64(q.Isize)-4:], uint32(v))
				}
			}

			p.Rel = nil

			p.Pc = int64(c)
			ab.asmins(pstate, ctxt, s, p)
			m := ab.Len()
			if int(p.Isize) != m {
				p.Isize = uint8(m)
				// When building for NaCl, we currently need
				// at least 2 rounds to ensure proper 32-byte alignment.
				if ctxt.Headtype == objabi.Hnacl {
					reAssemble = true
				}
			}

			s.Grow(p.Pc + int64(m))
			copy(s.P[p.Pc:], ab.Bytes())
			c += int32(m)
		}

		n++
		if n > 20 {
			ctxt.Diag("span must be looping")
			log.Fatalf("loop")
		}
		if !reAssemble {
			break
		}
		if ctxt.Errors > errors {
			return
		}
	}

	if ctxt.Headtype == objabi.Hnacl {
		c = pstate.naclpad(ctxt, s, c, -c&31)
	}

	s.Size = int64(c)

	if false { /* debug['a'] > 1 */
		fmt.Printf("span1 %s %d (%d tries)\n %.6x", s.Name, s.Size, n, 0)
		var i int
		for i = 0; i < len(s.P); i++ {
			fmt.Printf(" %.2x", s.P[i])
			if i%16 == 15 {
				fmt.Printf("\n  %.6x", uint(i+1))
			}
		}

		if i%16 != 0 {
			fmt.Printf("\n")
		}

		for i := 0; i < len(s.R); i++ {
			r := &s.R[i]
			fmt.Printf(" rel %#.4x/%d %s%+d\n", uint32(r.Off), r.Siz, r.Sym.Name, r.Add)
		}
	}
}

func (pstate *PackageState) instinit(ctxt *obj.Link) {
	if pstate.ycover[0] != 0 {
		// Already initialized; stop now.
		// This happens in the cmd/asm tests,
		// each of which re-initializes the arch.
		return
	}

	switch ctxt.Headtype {
	case objabi.Hplan9:
		pstate.plan9privates = ctxt.Lookup("_privates")
	case objabi.Hnacl:
		pstate.deferreturn = ctxt.Lookup("runtime.deferreturn")
	}

	for i := range pstate.avxOptab {
		c := pstate.avxOptab[i].as
		if pstate.opindex[c&obj.AMask] != nil {
			ctxt.Diag("phase error in avxOptab: %d (%v)", i, c)
		}
		pstate.opindex[c&obj.AMask] = &pstate.avxOptab[i]
	}
	for i := 1; pstate.optab[i].as != 0; i++ {
		c := pstate.optab[i].as
		if pstate.opindex[c&obj.AMask] != nil {
			ctxt.Diag("phase error in optab: %d (%v)", i, c)
		}
		pstate.opindex[c&obj.AMask] = &pstate.optab[i]
	}

	for i := 0; i < Ymax; i++ {
		pstate.ycover[i*Ymax+i] = 1
	}

	pstate.ycover[Yi0*Ymax+Yu2] = 1
	pstate.ycover[Yi1*Ymax+Yu2] = 1

	pstate.ycover[Yi0*Ymax+Yi8] = 1
	pstate.ycover[Yi1*Ymax+Yi8] = 1
	pstate.ycover[Yu2*Ymax+Yi8] = 1
	pstate.ycover[Yu7*Ymax+Yi8] = 1

	pstate.ycover[Yi0*Ymax+Yu7] = 1
	pstate.ycover[Yi1*Ymax+Yu7] = 1
	pstate.ycover[Yu2*Ymax+Yu7] = 1

	pstate.ycover[Yi0*Ymax+Yu8] = 1
	pstate.ycover[Yi1*Ymax+Yu8] = 1
	pstate.ycover[Yu2*Ymax+Yu8] = 1
	pstate.ycover[Yu7*Ymax+Yu8] = 1

	pstate.ycover[Yi0*Ymax+Ys32] = 1
	pstate.ycover[Yi1*Ymax+Ys32] = 1
	pstate.ycover[Yu2*Ymax+Ys32] = 1
	pstate.ycover[Yu7*Ymax+Ys32] = 1
	pstate.ycover[Yu8*Ymax+Ys32] = 1
	pstate.ycover[Yi8*Ymax+Ys32] = 1

	pstate.ycover[Yi0*Ymax+Yi32] = 1
	pstate.ycover[Yi1*Ymax+Yi32] = 1
	pstate.ycover[Yu2*Ymax+Yi32] = 1
	pstate.ycover[Yu7*Ymax+Yi32] = 1
	pstate.ycover[Yu8*Ymax+Yi32] = 1
	pstate.ycover[Yi8*Ymax+Yi32] = 1
	pstate.ycover[Ys32*Ymax+Yi32] = 1

	pstate.ycover[Yi0*Ymax+Yi64] = 1
	pstate.ycover[Yi1*Ymax+Yi64] = 1
	pstate.ycover[Yu7*Ymax+Yi64] = 1
	pstate.ycover[Yu2*Ymax+Yi64] = 1
	pstate.ycover[Yu8*Ymax+Yi64] = 1
	pstate.ycover[Yi8*Ymax+Yi64] = 1
	pstate.ycover[Ys32*Ymax+Yi64] = 1
	pstate.ycover[Yi32*Ymax+Yi64] = 1

	pstate.ycover[Yal*Ymax+Yrb] = 1
	pstate.ycover[Ycl*Ymax+Yrb] = 1
	pstate.ycover[Yax*Ymax+Yrb] = 1
	pstate.ycover[Ycx*Ymax+Yrb] = 1
	pstate.ycover[Yrx*Ymax+Yrb] = 1
	pstate.ycover[Yrl*Ymax+Yrb] = 1 // but not Yrl32

	pstate.ycover[Ycl*Ymax+Ycx] = 1

	pstate.ycover[Yax*Ymax+Yrx] = 1
	pstate.ycover[Ycx*Ymax+Yrx] = 1

	pstate.ycover[Yax*Ymax+Yrl] = 1
	pstate.ycover[Ycx*Ymax+Yrl] = 1
	pstate.ycover[Yrx*Ymax+Yrl] = 1
	pstate.ycover[Yrl32*Ymax+Yrl] = 1

	pstate.ycover[Yf0*Ymax+Yrf] = 1

	pstate.ycover[Yal*Ymax+Ymb] = 1
	pstate.ycover[Ycl*Ymax+Ymb] = 1
	pstate.ycover[Yax*Ymax+Ymb] = 1
	pstate.ycover[Ycx*Ymax+Ymb] = 1
	pstate.ycover[Yrx*Ymax+Ymb] = 1
	pstate.ycover[Yrb*Ymax+Ymb] = 1
	pstate.ycover[Yrl*Ymax+Ymb] = 1 // but not Yrl32
	pstate.ycover[Ym*Ymax+Ymb] = 1

	pstate.ycover[Yax*Ymax+Yml] = 1
	pstate.ycover[Ycx*Ymax+Yml] = 1
	pstate.ycover[Yrx*Ymax+Yml] = 1
	pstate.ycover[Yrl*Ymax+Yml] = 1
	pstate.ycover[Yrl32*Ymax+Yml] = 1
	pstate.ycover[Ym*Ymax+Yml] = 1

	pstate.ycover[Yax*Ymax+Ymm] = 1
	pstate.ycover[Ycx*Ymax+Ymm] = 1
	pstate.ycover[Yrx*Ymax+Ymm] = 1
	pstate.ycover[Yrl*Ymax+Ymm] = 1
	pstate.ycover[Yrl32*Ymax+Ymm] = 1
	pstate.ycover[Ym*Ymax+Ymm] = 1
	pstate.ycover[Ymr*Ymax+Ymm] = 1

	pstate.ycover[Yxr0*Ymax+Yxr] = 1

	pstate.ycover[Ym*Ymax+Yxm] = 1
	pstate.ycover[Yxr0*Ymax+Yxm] = 1
	pstate.ycover[Yxr*Ymax+Yxm] = 1

	pstate.ycover[Ym*Ymax+Yym] = 1
	pstate.ycover[Yyr*Ymax+Yym] = 1

	pstate.ycover[Yxr0*Ymax+YxrEvex] = 1
	pstate.ycover[Yxr*Ymax+YxrEvex] = 1

	pstate.ycover[Ym*Ymax+YxmEvex] = 1
	pstate.ycover[Yxr0*Ymax+YxmEvex] = 1
	pstate.ycover[Yxr*Ymax+YxmEvex] = 1
	pstate.ycover[YxrEvex*Ymax+YxmEvex] = 1

	pstate.ycover[Yyr*Ymax+YyrEvex] = 1

	pstate.ycover[Ym*Ymax+YymEvex] = 1
	pstate.ycover[Yyr*Ymax+YymEvex] = 1
	pstate.ycover[YyrEvex*Ymax+YymEvex] = 1

	pstate.ycover[Ym*Ymax+Yzm] = 1
	pstate.ycover[Yzr*Ymax+Yzm] = 1

	pstate.ycover[Yk0*Ymax+Yk] = 1
	pstate.ycover[Yknot0*Ymax+Yk] = 1

	pstate.ycover[Yk0*Ymax+Ykm] = 1
	pstate.ycover[Yknot0*Ymax+Ykm] = 1
	pstate.ycover[Yk*Ymax+Ykm] = 1
	pstate.ycover[Ym*Ymax+Ykm] = 1

	pstate.ycover[Yxvm*Ymax+YxvmEvex] = 1

	pstate.ycover[Yyvm*Ymax+YyvmEvex] = 1

	for i := 0; i < MAXREG; i++ {
		pstate.reg[i] = -1
		if i >= REG_AL && i <= REG_R15B {
			pstate.reg[i] = (i - REG_AL) & 7
			if i >= REG_SPB && i <= REG_DIB {
				pstate.regrex[i] = 0x40
			}
			if i >= REG_R8B && i <= REG_R15B {
				pstate.regrex[i] = Rxr | Rxx | Rxb
			}
		}

		if i >= REG_AH && i <= REG_BH {
			pstate.reg[i] = 4 + ((i - REG_AH) & 7)
		}
		if i >= REG_AX && i <= REG_R15 {
			pstate.reg[i] = (i - REG_AX) & 7
			if i >= REG_R8 {
				pstate.regrex[i] = Rxr | Rxx | Rxb
			}
		}

		if i >= REG_F0 && i <= REG_F0+7 {
			pstate.reg[i] = (i - REG_F0) & 7
		}
		if i >= REG_M0 && i <= REG_M0+7 {
			pstate.reg[i] = (i - REG_M0) & 7
		}
		if i >= REG_K0 && i <= REG_K0+7 {
			pstate.reg[i] = (i - REG_K0) & 7
		}
		if i >= REG_X0 && i <= REG_X0+15 {
			pstate.reg[i] = (i - REG_X0) & 7
			if i >= REG_X0+8 {
				pstate.regrex[i] = Rxr | Rxx | Rxb
			}
		}
		if i >= REG_X16 && i <= REG_X16+15 {
			pstate.reg[i] = (i - REG_X16) & 7
			if i >= REG_X16+8 {
				pstate.regrex[i] = Rxr | Rxx | Rxb | RxrEvex
			} else {
				pstate.regrex[i] = RxrEvex
			}
		}
		if i >= REG_Y0 && i <= REG_Y0+15 {
			pstate.reg[i] = (i - REG_Y0) & 7
			if i >= REG_Y0+8 {
				pstate.regrex[i] = Rxr | Rxx | Rxb
			}
		}
		if i >= REG_Y16 && i <= REG_Y16+15 {
			pstate.reg[i] = (i - REG_Y16) & 7
			if i >= REG_Y16+8 {
				pstate.regrex[i] = Rxr | Rxx | Rxb | RxrEvex
			} else {
				pstate.regrex[i] = RxrEvex
			}
		}
		if i >= REG_Z0 && i <= REG_Z0+15 {
			pstate.reg[i] = (i - REG_Z0) & 7
			if i > REG_Z0+7 {
				pstate.regrex[i] = Rxr | Rxx | Rxb
			}
		}
		if i >= REG_Z16 && i <= REG_Z16+15 {
			pstate.reg[i] = (i - REG_Z16) & 7
			if i >= REG_Z16+8 {
				pstate.regrex[i] = Rxr | Rxx | Rxb | RxrEvex
			} else {
				pstate.regrex[i] = RxrEvex
			}
		}

		if i >= REG_CR+8 && i <= REG_CR+15 {
			pstate.regrex[i] = Rxr
		}
	}
}

func (pstate *PackageState) prefixof(ctxt *obj.Link, a *obj.Addr) int {
	if a.Reg < REG_CS && a.Index < REG_CS { // fast path
		return 0
	}
	if a.Type == obj.TYPE_MEM && a.Name == obj.NAME_NONE {
		switch a.Reg {
		case REG_CS:
			return 0x2e

		case REG_DS:
			return 0x3e

		case REG_ES:
			return 0x26

		case REG_FS:
			return 0x64

		case REG_GS:
			return 0x65

		case REG_TLS:
			// NOTE: Systems listed here should be only systems that
			// support direct TLS references like 8(TLS) implemented as
			// direct references from FS or GS. Systems that require
			// the initial-exec model, where you load the TLS base into
			// a register and then index from that register, do not reach
			// this code and should not be listed.
			if ctxt.Arch.Family == sys.I386 {
				switch ctxt.Headtype {
				default:
					if pstate.isAndroid {
						return 0x65 // GS
					}
					log.Fatalf("unknown TLS base register for %v", ctxt.Headtype)

				case objabi.Hdarwin,
					objabi.Hdragonfly,
					objabi.Hfreebsd,
					objabi.Hnetbsd,
					objabi.Hopenbsd:
					return 0x65 // GS
				}
			}

			switch ctxt.Headtype {
			default:
				log.Fatalf("unknown TLS base register for %v", ctxt.Headtype)

			case objabi.Hlinux:
				if pstate.isAndroid {
					return 0x64 // FS
				}

				if ctxt.Flag_shared {
					log.Fatalf("unknown TLS base register for linux with -shared")
				} else {
					return 0x64 // FS
				}

			case objabi.Hdragonfly,
				objabi.Hfreebsd,
				objabi.Hnetbsd,
				objabi.Hopenbsd,
				objabi.Hsolaris:
				return 0x64 // FS

			case objabi.Hdarwin:
				return 0x65 // GS
			}
		}
	}

	if ctxt.Arch.Family == sys.I386 {
		if a.Index == REG_TLS && ctxt.Flag_shared {
			// When building for inclusion into a shared library, an instruction of the form
			//     MOVL 0(CX)(TLS*1), AX
			// becomes
			//     mov %gs:(%ecx), %eax
			// which assumes that the correct TLS offset has been loaded into %ecx (today
			// there is only one TLS variable -- g -- so this is OK). When not building for
			// a shared library the instruction it becomes
			//     mov 0x0(%ecx), $eax
			// and a R_TLS_LE relocation, and so does not require a prefix.
			if a.Offset != 0 {
				ctxt.Diag("cannot handle non-0 offsets to TLS")
			}
			return 0x65 // GS
		}
		return 0
	}

	switch a.Index {
	case REG_CS:
		return 0x2e

	case REG_DS:
		return 0x3e

	case REG_ES:
		return 0x26

	case REG_TLS:
		if ctxt.Flag_shared && ctxt.Headtype != objabi.Hwindows {
			// When building for inclusion into a shared library, an instruction of the form
			//     MOV 0(CX)(TLS*1), AX
			// becomes
			//     mov %fs:(%rcx), %rax
			// which assumes that the correct TLS offset has been loaded into %rcx (today
			// there is only one TLS variable -- g -- so this is OK). When not building for
			// a shared library the instruction does not require a prefix.
			if a.Offset != 0 {
				log.Fatalf("cannot handle non-0 offsets to TLS")
			}
			return 0x64
		}

	case REG_FS:
		return 0x64

	case REG_GS:
		return 0x65
	}

	return 0
}

// oclassRegList returns multisource operand class for addr.
func (pstate *PackageState) oclassRegList(ctxt *obj.Link, addr *obj.Addr) int {
	// TODO(quasilyte): when oclass register case is refactored into
	// lookup table, use it here to get register kind more easily.
	// Helper functions like regIsXmm should go away too (they will become redundant).

	regIsXmm := func(r int) bool { return r >= REG_X0 && r <= REG_X31 }
	regIsYmm := func(r int) bool { return r >= REG_Y0 && r <= REG_Y31 }
	regIsZmm := func(r int) bool { return r >= REG_Z0 && r <= REG_Z31 }

	reg0, reg1 := decodeRegisterRange(addr.Offset)
	low := pstate.regIndex(int16(reg0))
	high := pstate.regIndex(int16(reg1))

	if ctxt.Arch.Family == sys.I386 {
		if low >= 8 || high >= 8 {
			return Yxxx
		}
	}

	switch high - low {
	case 3:
		switch {
		case regIsXmm(reg0) && regIsXmm(reg1):
			return YxrEvexMulti4
		case regIsYmm(reg0) && regIsYmm(reg1):
			return YyrEvexMulti4
		case regIsZmm(reg0) && regIsZmm(reg1):
			return YzrMulti4
		default:
			return Yxxx
		}
	default:
		return Yxxx
	}
}

// oclassVMem returns V-mem (vector memory with VSIB) operand class.
// For addr that is not V-mem returns (Yxxx, false).
func oclassVMem(ctxt *obj.Link, addr *obj.Addr) (int, bool) {
	switch addr.Index {
	case REG_X0 + 0,
		REG_X0 + 1,
		REG_X0 + 2,
		REG_X0 + 3,
		REG_X0 + 4,
		REG_X0 + 5,
		REG_X0 + 6,
		REG_X0 + 7:
		return Yxvm, true
	case REG_X8 + 0,
		REG_X8 + 1,
		REG_X8 + 2,
		REG_X8 + 3,
		REG_X8 + 4,
		REG_X8 + 5,
		REG_X8 + 6,
		REG_X8 + 7:
		if ctxt.Arch.Family == sys.I386 {
			return Yxxx, true
		}
		return Yxvm, true
	case REG_X16 + 0,
		REG_X16 + 1,
		REG_X16 + 2,
		REG_X16 + 3,
		REG_X16 + 4,
		REG_X16 + 5,
		REG_X16 + 6,
		REG_X16 + 7,
		REG_X16 + 8,
		REG_X16 + 9,
		REG_X16 + 10,
		REG_X16 + 11,
		REG_X16 + 12,
		REG_X16 + 13,
		REG_X16 + 14,
		REG_X16 + 15:
		if ctxt.Arch.Family == sys.I386 {
			return Yxxx, true
		}
		return YxvmEvex, true

	case REG_Y0 + 0,
		REG_Y0 + 1,
		REG_Y0 + 2,
		REG_Y0 + 3,
		REG_Y0 + 4,
		REG_Y0 + 5,
		REG_Y0 + 6,
		REG_Y0 + 7:
		return Yyvm, true
	case REG_Y8 + 0,
		REG_Y8 + 1,
		REG_Y8 + 2,
		REG_Y8 + 3,
		REG_Y8 + 4,
		REG_Y8 + 5,
		REG_Y8 + 6,
		REG_Y8 + 7:
		if ctxt.Arch.Family == sys.I386 {
			return Yxxx, true
		}
		return Yyvm, true
	case REG_Y16 + 0,
		REG_Y16 + 1,
		REG_Y16 + 2,
		REG_Y16 + 3,
		REG_Y16 + 4,
		REG_Y16 + 5,
		REG_Y16 + 6,
		REG_Y16 + 7,
		REG_Y16 + 8,
		REG_Y16 + 9,
		REG_Y16 + 10,
		REG_Y16 + 11,
		REG_Y16 + 12,
		REG_Y16 + 13,
		REG_Y16 + 14,
		REG_Y16 + 15:
		if ctxt.Arch.Family == sys.I386 {
			return Yxxx, true
		}
		return YyvmEvex, true

	case REG_Z0 + 0,
		REG_Z0 + 1,
		REG_Z0 + 2,
		REG_Z0 + 3,
		REG_Z0 + 4,
		REG_Z0 + 5,
		REG_Z0 + 6,
		REG_Z0 + 7:
		return Yzvm, true
	case REG_Z8 + 0,
		REG_Z8 + 1,
		REG_Z8 + 2,
		REG_Z8 + 3,
		REG_Z8 + 4,
		REG_Z8 + 5,
		REG_Z8 + 6,
		REG_Z8 + 7,
		REG_Z8 + 8,
		REG_Z8 + 9,
		REG_Z8 + 10,
		REG_Z8 + 11,
		REG_Z8 + 12,
		REG_Z8 + 13,
		REG_Z8 + 14,
		REG_Z8 + 15,
		REG_Z8 + 16,
		REG_Z8 + 17,
		REG_Z8 + 18,
		REG_Z8 + 19,
		REG_Z8 + 20,
		REG_Z8 + 21,
		REG_Z8 + 22,
		REG_Z8 + 23:
		if ctxt.Arch.Family == sys.I386 {
			return Yxxx, true
		}
		return Yzvm, true
	}

	return Yxxx, false
}

func (pstate *PackageState) oclass(ctxt *obj.Link, p *obj.Prog, a *obj.Addr) int {
	switch a.Type {
	case obj.TYPE_REGLIST:
		return pstate.oclassRegList(ctxt, a)

	case obj.TYPE_NONE:
		return Ynone

	case obj.TYPE_BRANCH:
		return Ybr

	case obj.TYPE_INDIR:
		if a.Name != obj.NAME_NONE && a.Reg == REG_NONE && a.Index == REG_NONE && a.Scale == 0 {
			return Yindir
		}
		return Yxxx

	case obj.TYPE_MEM:
		// Pseudo registers have negative index, but SP is
		// not pseudo on x86, hence REG_SP check is not redundant.
		if a.Index == REG_SP || a.Index < 0 {
			// Can't use FP/SB/PC/SP as the index register.
			return Yxxx
		}

		if vmem, ok := oclassVMem(ctxt, a); ok {
			return vmem
		}

		if ctxt.Arch.Family == sys.AMD64 {
			switch a.Name {
			case obj.NAME_EXTERN, obj.NAME_STATIC, obj.NAME_GOTREF:
				// Global variables can't use index registers and their
				// base register is %rip (%rip is encoded as REG_NONE).
				if a.Reg != REG_NONE || a.Index != REG_NONE || a.Scale != 0 {
					return Yxxx
				}
			case obj.NAME_AUTO, obj.NAME_PARAM:
				// These names must have a base of SP.  The old compiler
				// uses 0 for the base register. SSA uses REG_SP.
				if a.Reg != REG_SP && a.Reg != 0 {
					return Yxxx
				}
			case obj.NAME_NONE:
			// everything is ok
			default:
				// unknown name
				return Yxxx
			}
		}
		return Ym

	case obj.TYPE_ADDR:
		switch a.Name {
		case obj.NAME_GOTREF:
			ctxt.Diag("unexpected TYPE_ADDR with NAME_GOTREF")
			return Yxxx

		case obj.NAME_EXTERN,
			obj.NAME_STATIC:
			if a.Sym != nil && useAbs(ctxt, a.Sym) {
				return Yi32
			}
			return Yiauto // use pc-relative addressing

		case obj.NAME_AUTO,
			obj.NAME_PARAM:
			return Yiauto
		}

		// TODO(rsc): DUFFZERO/DUFFCOPY encoding forgot to set a->index
		// and got Yi32 in an earlier version of this code.
		// Keep doing that until we fix yduff etc.
		if a.Sym != nil && strings.HasPrefix(a.Sym.Name, "runtime.duff") {
			return Yi32
		}

		if a.Sym != nil || a.Name != obj.NAME_NONE {
			ctxt.Diag("unexpected addr: %v", pstate.obj.Dconv(p, a))
		}
		fallthrough

	case obj.TYPE_CONST:
		if a.Sym != nil {
			ctxt.Diag("TYPE_CONST with symbol: %v", pstate.obj.Dconv(p, a))
		}

		v := a.Offset
		if ctxt.Arch.Family == sys.I386 {
			v = int64(int32(v))
		}
		switch {
		case v == 0:
			return Yi0
		case v == 1:
			return Yi1
		case v >= 0 && v <= 3:
			return Yu2
		case v >= 0 && v <= 127:
			return Yu7
		case v >= 0 && v <= 255:
			return Yu8
		case v >= -128 && v <= 127:
			return Yi8
		}
		if ctxt.Arch.Family == sys.I386 {
			return Yi32
		}
		l := int32(v)
		if int64(l) == v {
			return Ys32 // can sign extend
		}
		if v>>32 == 0 {
			return Yi32 // unsigned
		}
		return Yi64

	case obj.TYPE_TEXTSIZE:
		return Ytextsize
	}

	if a.Type != obj.TYPE_REG {
		ctxt.Diag("unexpected addr1: type=%d %v", a.Type, pstate.obj.Dconv(p, a))
		return Yxxx
	}

	switch a.Reg {
	case REG_AL:
		return Yal

	case REG_AX:
		return Yax

	/*
		case REG_SPB:
	*/
	case REG_BPB,
		REG_SIB,
		REG_DIB,
		REG_R8B,
		REG_R9B,
		REG_R10B,
		REG_R11B,
		REG_R12B,
		REG_R13B,
		REG_R14B,
		REG_R15B:
		if ctxt.Arch.Family == sys.I386 {
			return Yxxx
		}
		fallthrough

	case REG_DL,
		REG_BL,
		REG_AH,
		REG_CH,
		REG_DH,
		REG_BH:
		return Yrb

	case REG_CL:
		return Ycl

	case REG_CX:
		return Ycx

	case REG_DX, REG_BX:
		return Yrx

	case REG_R8, // not really Yrl
		REG_R9,
		REG_R10,
		REG_R11,
		REG_R12,
		REG_R13,
		REG_R14,
		REG_R15:
		if ctxt.Arch.Family == sys.I386 {
			return Yxxx
		}
		fallthrough

	case REG_SP, REG_BP, REG_SI, REG_DI:
		if ctxt.Arch.Family == sys.I386 {
			return Yrl32
		}
		return Yrl

	case REG_F0 + 0:
		return Yf0

	case REG_F0 + 1,
		REG_F0 + 2,
		REG_F0 + 3,
		REG_F0 + 4,
		REG_F0 + 5,
		REG_F0 + 6,
		REG_F0 + 7:
		return Yrf

	case REG_M0 + 0,
		REG_M0 + 1,
		REG_M0 + 2,
		REG_M0 + 3,
		REG_M0 + 4,
		REG_M0 + 5,
		REG_M0 + 6,
		REG_M0 + 7:
		return Ymr

	case REG_X0:
		return Yxr0

	case REG_X0 + 1,
		REG_X0 + 2,
		REG_X0 + 3,
		REG_X0 + 4,
		REG_X0 + 5,
		REG_X0 + 6,
		REG_X0 + 7,
		REG_X0 + 8,
		REG_X0 + 9,
		REG_X0 + 10,
		REG_X0 + 11,
		REG_X0 + 12,
		REG_X0 + 13,
		REG_X0 + 14,
		REG_X0 + 15:
		return Yxr

	case REG_X0 + 16,
		REG_X0 + 17,
		REG_X0 + 18,
		REG_X0 + 19,
		REG_X0 + 20,
		REG_X0 + 21,
		REG_X0 + 22,
		REG_X0 + 23,
		REG_X0 + 24,
		REG_X0 + 25,
		REG_X0 + 26,
		REG_X0 + 27,
		REG_X0 + 28,
		REG_X0 + 29,
		REG_X0 + 30,
		REG_X0 + 31:
		return YxrEvex

	case REG_Y0 + 0,
		REG_Y0 + 1,
		REG_Y0 + 2,
		REG_Y0 + 3,
		REG_Y0 + 4,
		REG_Y0 + 5,
		REG_Y0 + 6,
		REG_Y0 + 7,
		REG_Y0 + 8,
		REG_Y0 + 9,
		REG_Y0 + 10,
		REG_Y0 + 11,
		REG_Y0 + 12,
		REG_Y0 + 13,
		REG_Y0 + 14,
		REG_Y0 + 15:
		return Yyr

	case REG_Y0 + 16,
		REG_Y0 + 17,
		REG_Y0 + 18,
		REG_Y0 + 19,
		REG_Y0 + 20,
		REG_Y0 + 21,
		REG_Y0 + 22,
		REG_Y0 + 23,
		REG_Y0 + 24,
		REG_Y0 + 25,
		REG_Y0 + 26,
		REG_Y0 + 27,
		REG_Y0 + 28,
		REG_Y0 + 29,
		REG_Y0 + 30,
		REG_Y0 + 31:
		return YyrEvex

	case REG_Z0 + 0,
		REG_Z0 + 1,
		REG_Z0 + 2,
		REG_Z0 + 3,
		REG_Z0 + 4,
		REG_Z0 + 5,
		REG_Z0 + 6,
		REG_Z0 + 7:
		return Yzr

	case REG_Z0 + 8,
		REG_Z0 + 9,
		REG_Z0 + 10,
		REG_Z0 + 11,
		REG_Z0 + 12,
		REG_Z0 + 13,
		REG_Z0 + 14,
		REG_Z0 + 15,
		REG_Z0 + 16,
		REG_Z0 + 17,
		REG_Z0 + 18,
		REG_Z0 + 19,
		REG_Z0 + 20,
		REG_Z0 + 21,
		REG_Z0 + 22,
		REG_Z0 + 23,
		REG_Z0 + 24,
		REG_Z0 + 25,
		REG_Z0 + 26,
		REG_Z0 + 27,
		REG_Z0 + 28,
		REG_Z0 + 29,
		REG_Z0 + 30,
		REG_Z0 + 31:
		if ctxt.Arch.Family == sys.I386 {
			return Yxxx
		}
		return Yzr

	case REG_K0:
		return Yk0

	case REG_K0 + 1,
		REG_K0 + 2,
		REG_K0 + 3,
		REG_K0 + 4,
		REG_K0 + 5,
		REG_K0 + 6,
		REG_K0 + 7:
		return Yknot0

	case REG_CS:
		return Ycs
	case REG_SS:
		return Yss
	case REG_DS:
		return Yds
	case REG_ES:
		return Yes
	case REG_FS:
		return Yfs
	case REG_GS:
		return Ygs
	case REG_TLS:
		return Ytls

	case REG_GDTR:
		return Ygdtr
	case REG_IDTR:
		return Yidtr
	case REG_LDTR:
		return Yldtr
	case REG_MSW:
		return Ymsw
	case REG_TASK:
		return Ytask

	case REG_CR + 0:
		return Ycr0
	case REG_CR + 1:
		return Ycr1
	case REG_CR + 2:
		return Ycr2
	case REG_CR + 3:
		return Ycr3
	case REG_CR + 4:
		return Ycr4
	case REG_CR + 5:
		return Ycr5
	case REG_CR + 6:
		return Ycr6
	case REG_CR + 7:
		return Ycr7
	case REG_CR + 8:
		return Ycr8

	case REG_DR + 0:
		return Ydr0
	case REG_DR + 1:
		return Ydr1
	case REG_DR + 2:
		return Ydr2
	case REG_DR + 3:
		return Ydr3
	case REG_DR + 4:
		return Ydr4
	case REG_DR + 5:
		return Ydr5
	case REG_DR + 6:
		return Ydr6
	case REG_DR + 7:
		return Ydr7

	case REG_TR + 0:
		return Ytr0
	case REG_TR + 1:
		return Ytr1
	case REG_TR + 2:
		return Ytr2
	case REG_TR + 3:
		return Ytr3
	case REG_TR + 4:
		return Ytr4
	case REG_TR + 5:
		return Ytr5
	case REG_TR + 6:
		return Ytr6
	case REG_TR + 7:
		return Ytr7
	}

	return Yxxx
}

// AsmBuf is a simple buffer to assemble variable-length x86 instructions into
// and hold assembly state.
type AsmBuf struct {
	buf      [100]byte
	off      int
	rexflag  int
	vexflag  bool // Per inst: true for VEX-encoded
	evexflag bool // Per inst: true for EVEX-encoded
	rep      bool
	repn     bool
	lock     bool

	evex evexBits // Initialized when evexflag is true
}

// Put1 appends one byte to the end of the buffer.
func (ab *AsmBuf) Put1(x byte) {
	ab.buf[ab.off] = x
	ab.off++
}

// Put2 appends two bytes to the end of the buffer.
func (ab *AsmBuf) Put2(x, y byte) {
	ab.buf[ab.off+0] = x
	ab.buf[ab.off+1] = y
	ab.off += 2
}

// Put3 appends three bytes to the end of the buffer.
func (ab *AsmBuf) Put3(x, y, z byte) {
	ab.buf[ab.off+0] = x
	ab.buf[ab.off+1] = y
	ab.buf[ab.off+2] = z
	ab.off += 3
}

// Put4 appends four bytes to the end of the buffer.
func (ab *AsmBuf) Put4(x, y, z, w byte) {
	ab.buf[ab.off+0] = x
	ab.buf[ab.off+1] = y
	ab.buf[ab.off+2] = z
	ab.buf[ab.off+3] = w
	ab.off += 4
}

// PutInt16 writes v into the buffer using little-endian encoding.
func (ab *AsmBuf) PutInt16(v int16) {
	ab.buf[ab.off+0] = byte(v)
	ab.buf[ab.off+1] = byte(v >> 8)
	ab.off += 2
}

// PutInt32 writes v into the buffer using little-endian encoding.
func (ab *AsmBuf) PutInt32(v int32) {
	ab.buf[ab.off+0] = byte(v)
	ab.buf[ab.off+1] = byte(v >> 8)
	ab.buf[ab.off+2] = byte(v >> 16)
	ab.buf[ab.off+3] = byte(v >> 24)
	ab.off += 4
}

// PutInt64 writes v into the buffer using little-endian encoding.
func (ab *AsmBuf) PutInt64(v int64) {
	ab.buf[ab.off+0] = byte(v)
	ab.buf[ab.off+1] = byte(v >> 8)
	ab.buf[ab.off+2] = byte(v >> 16)
	ab.buf[ab.off+3] = byte(v >> 24)
	ab.buf[ab.off+4] = byte(v >> 32)
	ab.buf[ab.off+5] = byte(v >> 40)
	ab.buf[ab.off+6] = byte(v >> 48)
	ab.buf[ab.off+7] = byte(v >> 56)
	ab.off += 8
}

// Put copies b into the buffer.
func (ab *AsmBuf) Put(b []byte) {
	copy(ab.buf[ab.off:], b)
	ab.off += len(b)
}

// PutOpBytesLit writes zero terminated sequence of bytes from op,
// starting at specified offsed (e.g. z counter value).
// Trailing 0 is not written.
//
// Intended to be used for literal Z cases.
// Literal Z cases usually have "Zlit" in their name (Zlit, Zlitr_m, Zlitm_r).
func (ab *AsmBuf) PutOpBytesLit(offset int, op *opBytes) {
	for int(op[offset]) != 0 {
		ab.Put1(byte(op[offset]))
		offset++
	}
}

// Insert inserts b at offset i.
func (ab *AsmBuf) Insert(i int, b byte) {
	ab.off++
	copy(ab.buf[i+1:ab.off], ab.buf[i:ab.off-1])
	ab.buf[i] = b
}

// Last returns the byte at the end of the buffer.
func (ab *AsmBuf) Last() byte { return ab.buf[ab.off-1] }

// Len returns the length of the buffer.
func (ab *AsmBuf) Len() int { return ab.off }

// Bytes returns the contents of the buffer.
func (ab *AsmBuf) Bytes() []byte { return ab.buf[:ab.off] }

// Reset empties the buffer.
func (ab *AsmBuf) Reset() { ab.off = 0 }

// At returns the byte at offset i.
func (ab *AsmBuf) At(i int) byte { return ab.buf[i] }

// asmidx emits SIB byte.
func (ab *AsmBuf) asmidx(pstate *PackageState, ctxt *obj.Link, scale int, index int, base int) {
	var i int

	// X/Y index register is used in VSIB.
	switch index {
	default:
		goto bad

	case REG_NONE:
		i = 4 << 3
		goto bas

	case REG_R8,
		REG_R9,
		REG_R10,
		REG_R11,
		REG_R12,
		REG_R13,
		REG_R14,
		REG_R15,
		REG_X8,
		REG_X9,
		REG_X10,
		REG_X11,
		REG_X12,
		REG_X13,
		REG_X14,
		REG_X15,
		REG_X16,
		REG_X17,
		REG_X18,
		REG_X19,
		REG_X20,
		REG_X21,
		REG_X22,
		REG_X23,
		REG_X24,
		REG_X25,
		REG_X26,
		REG_X27,
		REG_X28,
		REG_X29,
		REG_X30,
		REG_X31,
		REG_Y8,
		REG_Y9,
		REG_Y10,
		REG_Y11,
		REG_Y12,
		REG_Y13,
		REG_Y14,
		REG_Y15,
		REG_Y16,
		REG_Y17,
		REG_Y18,
		REG_Y19,
		REG_Y20,
		REG_Y21,
		REG_Y22,
		REG_Y23,
		REG_Y24,
		REG_Y25,
		REG_Y26,
		REG_Y27,
		REG_Y28,
		REG_Y29,
		REG_Y30,
		REG_Y31,
		REG_Z8,
		REG_Z9,
		REG_Z10,
		REG_Z11,
		REG_Z12,
		REG_Z13,
		REG_Z14,
		REG_Z15,
		REG_Z16,
		REG_Z17,
		REG_Z18,
		REG_Z19,
		REG_Z20,
		REG_Z21,
		REG_Z22,
		REG_Z23,
		REG_Z24,
		REG_Z25,
		REG_Z26,
		REG_Z27,
		REG_Z28,
		REG_Z29,
		REG_Z30,
		REG_Z31:
		if ctxt.Arch.Family == sys.I386 {
			goto bad
		}
		fallthrough

	case REG_AX,
		REG_CX,
		REG_DX,
		REG_BX,
		REG_BP,
		REG_SI,
		REG_DI,
		REG_X0,
		REG_X1,
		REG_X2,
		REG_X3,
		REG_X4,
		REG_X5,
		REG_X6,
		REG_X7,
		REG_Y0,
		REG_Y1,
		REG_Y2,
		REG_Y3,
		REG_Y4,
		REG_Y5,
		REG_Y6,
		REG_Y7,
		REG_Z0,
		REG_Z1,
		REG_Z2,
		REG_Z3,
		REG_Z4,
		REG_Z5,
		REG_Z6,
		REG_Z7:
		i = pstate.reg[index] << 3
	}

	switch scale {
	default:
		goto bad

	case 1:
		break

	case 2:
		i |= 1 << 6

	case 4:
		i |= 2 << 6

	case 8:
		i |= 3 << 6
	}

bas:
	switch base {
	default:
		goto bad

	case REG_NONE: // must be mod=00
		i |= 5

	case REG_R8,
		REG_R9,
		REG_R10,
		REG_R11,
		REG_R12,
		REG_R13,
		REG_R14,
		REG_R15:
		if ctxt.Arch.Family == sys.I386 {
			goto bad
		}
		fallthrough

	case REG_AX,
		REG_CX,
		REG_DX,
		REG_BX,
		REG_SP,
		REG_BP,
		REG_SI,
		REG_DI:
		i |= pstate.reg[base]
	}

	ab.Put1(byte(i))
	return

bad:
	ctxt.Diag("asmidx: bad address %d/%d/%d", scale, index, base)
	ab.Put1(0)
}

func (ab *AsmBuf) relput4(pstate *PackageState, ctxt *obj.Link, cursym *obj.LSym, p *obj.Prog, a *obj.Addr) {
	var rel obj.Reloc

	v := pstate.vaddr(ctxt, p, a, &rel)
	if rel.Siz != 0 {
		if rel.Siz != 4 {
			ctxt.Diag("bad reloc")
		}
		r := obj.Addrel(cursym)
		*r = rel
		r.Off = int32(p.Pc + int64(ab.Len()))
	}

	ab.PutInt32(int32(v))
}

func (pstate *PackageState) vaddr(ctxt *obj.Link, p *obj.Prog, a *obj.Addr, r *obj.Reloc) int64 {
	if r != nil {
		*r = obj.Reloc{}
	}

	switch a.Name {
	case obj.NAME_STATIC,
		obj.NAME_GOTREF,
		obj.NAME_EXTERN:
		s := a.Sym
		if r == nil {
			ctxt.Diag("need reloc for %v", pstate.obj.Dconv(p, a))
			log.Fatalf("reloc")
		}

		if a.Name == obj.NAME_GOTREF {
			r.Siz = 4
			r.Type = objabi.R_GOTPCREL
		} else if useAbs(ctxt, s) {
			r.Siz = 4
			r.Type = objabi.R_ADDR
		} else {
			r.Siz = 4
			r.Type = objabi.R_PCREL
		}

		r.Off = -1 // caller must fill in
		r.Sym = s
		r.Add = a.Offset

		return 0
	}

	if (a.Type == obj.TYPE_MEM || a.Type == obj.TYPE_ADDR) && a.Reg == REG_TLS {
		if r == nil {
			ctxt.Diag("need reloc for %v", pstate.obj.Dconv(p, a))
			log.Fatalf("reloc")
		}

		if !ctxt.Flag_shared || pstate.isAndroid || ctxt.Headtype == objabi.Hdarwin {
			r.Type = objabi.R_TLS_LE
			r.Siz = 4
			r.Off = -1 // caller must fill in
			r.Add = a.Offset
		}
		return 0
	}

	return a.Offset
}

func (ab *AsmBuf) asmandsz(pstate *PackageState, ctxt *obj.Link, cursym *obj.LSym, p *obj.Prog, a *obj.Addr, r int, rex int, m64 int) {
	var base int
	var rel obj.Reloc

	rex &= 0x40 | Rxr
	if a.Offset != int64(int32(a.Offset)) {
		// The rules are slightly different for 386 and AMD64,
		// mostly for historical reasons. We may unify them later,
		// but it must be discussed beforehand.
		//
		// For 64bit mode only LEAL is allowed to overflow.
		// It's how https://golang.org/cl/59630 made it.
		// crypto/sha1/sha1block_amd64.s depends on this feature.
		//
		// For 32bit mode rules are more permissive.
		// If offset fits uint32, it's permitted.
		// This is allowed for assembly that wants to use 32-bit hex
		// constants, e.g. LEAL 0x99999999(AX), AX.
		overflowOK := (ctxt.Arch.Family == sys.AMD64 && p.As == ALEAL) ||
			(ctxt.Arch.Family != sys.AMD64 &&
				int64(uint32(a.Offset)) == a.Offset &&
				ab.rexflag&Rxw == 0)
		if !overflowOK {
			ctxt.Diag("offset too large in %s", p)
		}
	}
	v := int32(a.Offset)
	rel.Siz = 0

	switch a.Type {
	case obj.TYPE_ADDR:
		if a.Name == obj.NAME_NONE {
			ctxt.Diag("unexpected TYPE_ADDR with NAME_NONE")
		}
		if a.Index == REG_TLS {
			ctxt.Diag("unexpected TYPE_ADDR with index==REG_TLS")
		}
		goto bad

	case obj.TYPE_REG:
		const regFirst = REG_AL
		const regLast = REG_Z31
		if a.Reg < regFirst || regLast < a.Reg {
			goto bad
		}
		if v != 0 {
			goto bad
		}
		ab.Put1(byte(3<<6 | pstate.reg[a.Reg]<<0 | r<<3))
		ab.rexflag |= pstate.regrex[a.Reg]&(0x40|Rxb) | rex
		return
	}

	if a.Type != obj.TYPE_MEM {
		goto bad
	}

	if a.Index != REG_NONE && a.Index != REG_TLS {
		base := int(a.Reg)
		switch a.Name {
		case obj.NAME_EXTERN,
			obj.NAME_GOTREF,
			obj.NAME_STATIC:
			if !useAbs(ctxt, a.Sym) && ctxt.Arch.Family == sys.AMD64 {
				goto bad
			}
			if ctxt.Arch.Family == sys.I386 && ctxt.Flag_shared {
				// The base register has already been set. It holds the PC
				// of this instruction returned by a PC-reading thunk.
				// See obj6.go:rewriteToPcrel.
			} else {
				base = REG_NONE
			}
			v = int32(pstate.vaddr(ctxt, p, a, &rel))

		case obj.NAME_AUTO,
			obj.NAME_PARAM:
			base = REG_SP
		}

		ab.rexflag |= pstate.regrex[int(a.Index)]&Rxx | pstate.regrex[base]&Rxb | rex
		if base == REG_NONE {
			ab.Put1(byte(0<<6 | 4<<0 | r<<3))
			ab.asmidx(pstate, ctxt, int(a.Scale), int(a.Index), base)
			goto putrelv
		}

		if v == 0 && rel.Siz == 0 && base != REG_BP && base != REG_R13 {
			ab.Put1(byte(0<<6 | 4<<0 | r<<3))
			ab.asmidx(pstate, ctxt, int(a.Scale), int(a.Index), base)
			return
		}

		if disp8, ok := pstate.toDisp8(v, p, ab); ok && rel.Siz == 0 {
			ab.Put1(byte(1<<6 | 4<<0 | r<<3))
			ab.asmidx(pstate, ctxt, int(a.Scale), int(a.Index), base)
			ab.Put1(disp8)
			return
		}

		ab.Put1(byte(2<<6 | 4<<0 | r<<3))
		ab.asmidx(pstate, ctxt, int(a.Scale), int(a.Index), base)
		goto putrelv
	}

	base = int(a.Reg)
	switch a.Name {
	case obj.NAME_STATIC,
		obj.NAME_GOTREF,
		obj.NAME_EXTERN:
		if a.Sym == nil {
			ctxt.Diag("bad addr: %v", p)
		}
		if ctxt.Arch.Family == sys.I386 && ctxt.Flag_shared {
			// The base register has already been set. It holds the PC
			// of this instruction returned by a PC-reading thunk.
			// See obj6.go:rewriteToPcrel.
		} else {
			base = REG_NONE
		}
		v = int32(pstate.vaddr(ctxt, p, a, &rel))

	case obj.NAME_AUTO,
		obj.NAME_PARAM:
		base = REG_SP
	}

	if base == REG_TLS {
		v = int32(pstate.vaddr(ctxt, p, a, &rel))
	}

	ab.rexflag |= pstate.regrex[base]&Rxb | rex
	if base == REG_NONE || (REG_CS <= base && base <= REG_GS) || base == REG_TLS {
		if (a.Sym == nil || !useAbs(ctxt, a.Sym)) && base == REG_NONE && (a.Name == obj.NAME_STATIC || a.Name == obj.NAME_EXTERN || a.Name == obj.NAME_GOTREF) || ctxt.Arch.Family != sys.AMD64 {
			if a.Name == obj.NAME_GOTREF && (a.Offset != 0 || a.Index != 0 || a.Scale != 0) {
				ctxt.Diag("%v has offset against gotref", p)
			}
			ab.Put1(byte(0<<6 | 5<<0 | r<<3))
			goto putrelv
		}

		// temporary
		ab.Put2(
			byte(0<<6|4<<0|r<<3), // sib present
			0<<6|4<<3|5<<0,       // DS:d32
		)
		goto putrelv
	}

	if base == REG_SP || base == REG_R12 {
		if v == 0 {
			ab.Put1(byte(0<<6 | pstate.reg[base]<<0 | r<<3))
			ab.asmidx(pstate, ctxt, int(a.Scale), REG_NONE, base)
			return
		}

		if disp8, ok := pstate.toDisp8(v, p, ab); ok {
			ab.Put1(byte(1<<6 | pstate.reg[base]<<0 | r<<3))
			ab.asmidx(pstate, ctxt, int(a.Scale), REG_NONE, base)
			ab.Put1(disp8)
			return
		}

		ab.Put1(byte(2<<6 | pstate.reg[base]<<0 | r<<3))
		ab.asmidx(pstate, ctxt, int(a.Scale), REG_NONE, base)
		goto putrelv
	}

	if REG_AX <= base && base <= REG_R15 {
		if a.Index == REG_TLS && !ctxt.Flag_shared {
			rel = obj.Reloc{}
			rel.Type = objabi.R_TLS_LE
			rel.Siz = 4
			rel.Sym = nil
			rel.Add = int64(v)
			v = 0
		}

		if v == 0 && rel.Siz == 0 && base != REG_BP && base != REG_R13 {
			ab.Put1(byte(0<<6 | pstate.reg[base]<<0 | r<<3))
			return
		}

		if disp8, ok := pstate.toDisp8(v, p, ab); ok && rel.Siz == 0 {
			ab.Put2(byte(1<<6|pstate.reg[base]<<0|r<<3), disp8)
			return
		}

		ab.Put1(byte(2<<6 | pstate.reg[base]<<0 | r<<3))
		goto putrelv
	}

	goto bad

putrelv:
	if rel.Siz != 0 {
		if rel.Siz != 4 {
			ctxt.Diag("bad rel")
			goto bad
		}

		r := obj.Addrel(cursym)
		*r = rel
		r.Off = int32(p.Pc + int64(ab.Len()))
	}

	ab.PutInt32(v)
	return

bad:
	ctxt.Diag("asmand: bad address %v", pstate.obj.Dconv(p, a))
}

func (ab *AsmBuf) asmand(pstate *PackageState, ctxt *obj.Link, cursym *obj.LSym, p *obj.Prog, a *obj.Addr, ra *obj.Addr) {
	ab.asmandsz(pstate, ctxt, cursym, p, a, pstate.reg[ra.Reg], pstate.regrex[ra.Reg], 0)
}

func (ab *AsmBuf) asmando(pstate *PackageState, ctxt *obj.Link, cursym *obj.LSym, p *obj.Prog, a *obj.Addr, o int) {
	ab.asmandsz(pstate, ctxt, cursym, p, a, o, 0, 0)
}

func bytereg(a *obj.Addr, t *uint8) {
	if a.Type == obj.TYPE_REG && a.Index == REG_NONE && (REG_AX <= a.Reg && a.Reg <= REG_R15) {
		a.Reg += REG_AL - REG_AX
		*t = 0
	}
}

func unbytereg(a *obj.Addr, t *uint8) {
	if a.Type == obj.TYPE_REG && a.Index == REG_NONE && (REG_AL <= a.Reg && a.Reg <= REG_R15B) {
		a.Reg += REG_AX - REG_AL
		*t = 0
	}
}

const (
	movLit uint8 = iota // Like Zlit
	movRegMem
	movMemReg
	movRegMem2op
	movMemReg2op
	movFullPtr // Load full pointer, trash heap (unsupported)
	movDoubleShift
	movTLSReg
)

func isax(a *obj.Addr) bool {
	switch a.Reg {
	case REG_AX, REG_AL, REG_AH:
		return true
	}

	if a.Index == REG_AX {
		return true
	}
	return false
}

func (pstate *PackageState) subreg(p *obj.Prog, from int, to int) {
	if false { /* debug['Q'] */
		fmt.Printf("\n%v\ts/%v/%v/\n", p, pstate.rconv(from), pstate.rconv(to))
	}

	if int(p.From.Reg) == from {
		p.From.Reg = int16(to)
		p.Ft = 0
	}

	if int(p.To.Reg) == from {
		p.To.Reg = int16(to)
		p.Tt = 0
	}

	if int(p.From.Index) == from {
		p.From.Index = int16(to)
		p.Ft = 0
	}

	if int(p.To.Index) == from {
		p.To.Index = int16(to)
		p.Tt = 0
	}

	if false { /* debug['Q'] */
		fmt.Printf("%v\n", p)
	}
}

func (ab *AsmBuf) mediaop(ctxt *obj.Link, o *Optab, op int, osize int, z int) int {
	switch op {
	case Pm, Pe, Pf2, Pf3:
		if osize != 1 {
			if op != Pm {
				ab.Put1(byte(op))
			}
			ab.Put1(Pm)
			z++
			op = int(o.op[z])
			break
		}
		fallthrough

	default:
		if ab.Len() == 0 || ab.Last() != Pm {
			ab.Put1(Pm)
		}
	}

	ab.Put1(byte(op))
	return z
}

// asmevex emits EVEX pregis and opcode byte.
// In addition to asmvex r/m, vvvv and reg fields also requires optional
// K-masking register.
//
// Expects asmbuf.evex to be properly initialized.
func (ab *AsmBuf) asmevex(pstate *PackageState, ctxt *obj.Link, p *obj.Prog, rm, v, r, k *obj.Addr) {
	ab.evexflag = true
	evex := ab.evex

	rexR := byte(1)
	evexR := byte(1)
	rexX := byte(1)
	rexB := byte(1)
	if r != nil {
		if pstate.regrex[r.Reg]&Rxr != 0 {
			rexR = 0 // "ModR/M.reg" selector 4th bit.
		}
		if pstate.regrex[r.Reg]&RxrEvex != 0 {
			evexR = 0 // "ModR/M.reg" selector 5th bit.
		}
	}
	if rm != nil {
		if rm.Index == REG_NONE && pstate.regrex[rm.Reg]&RxrEvex != 0 {
			rexX = 0
		} else if pstate.regrex[rm.Index]&Rxx != 0 {
			rexX = 0
		}
		if pstate.regrex[rm.Reg]&Rxb != 0 {
			rexB = 0
		}
	}
	// P0 = [R][X][B][R'][00][mm]
	p0 := (rexR << 7) |
		(rexX << 6) |
		(rexB << 5) |
		(evexR << 4) |
		(0 << 2) |
		(evex.M() << 0)

	vexV := byte(0)
	if v != nil {
		// 4bit-wide reg index.
		vexV = byte(pstate.reg[v.Reg]|(pstate.regrex[v.Reg]&Rxr)<<1) & 0xF
	}
	vexV ^= 0x0F
	// P1 = [W][vvvv][1][pp]
	p1 := (evex.W() << 7) |
		(vexV << 3) |
		(1 << 2) |
		(evex.P() << 0)

	suffix := pstate.evexSuffixMap[p.Scond]
	evexZ := byte(0)
	evexLL := evex.L()
	evexB := byte(0)
	evexV := byte(1)
	evexA := byte(0)
	if suffix.zeroing {
		if !evex.ZeroingEnabled() {
			ctxt.Diag("unsupported zeroing: %v", p)
		}
		evexZ = 1
	}
	switch {
	case suffix.rounding != rcUnset:
		if rm != nil && rm.Type == obj.TYPE_MEM {
			ctxt.Diag("illegal rounding with memory argument: %v", p)
		} else if !evex.RoundingEnabled() {
			ctxt.Diag("unsupported rounding: %v", p)
		}
		evexB = 1
		evexLL = suffix.rounding
	case suffix.broadcast:
		if rm == nil || rm.Type != obj.TYPE_MEM {
			ctxt.Diag("illegal broadcast without memory argument: %v", p)
		} else if !evex.BroadcastEnabled() {
			ctxt.Diag("unsupported broadcast: %v", p)
		}
		evexB = 1
	case suffix.sae:
		if rm != nil && rm.Type == obj.TYPE_MEM {
			ctxt.Diag("illegal SAE with memory argument: %v", p)
		} else if !evex.SaeEnabled() {
			ctxt.Diag("unsupported SAE: %v", p)
		}
		evexB = 1
	}
	if rm != nil && pstate.regrex[rm.Index]&RxrEvex != 0 {
		evexV = 0
	} else if v != nil && pstate.regrex[v.Reg]&RxrEvex != 0 {
		evexV = 0 // VSR selector 5th bit.
	}
	if k != nil {
		evexA = byte(pstate.reg[k.Reg])
	}
	// P2 = [z][L'L][b][V'][aaa]
	p2 := (evexZ << 7) |
		(evexLL << 5) |
		(evexB << 4) |
		(evexV << 3) |
		(evexA << 0)

	const evexEscapeByte = 0x62
	ab.Put4(evexEscapeByte, p0, p1, p2)
	ab.Put1(evex.opcode)
}

// Emit VEX prefix and opcode byte.
// The three addresses are the r/m, vvvv, and reg fields.
// The reg and rm arguments appear in the same order as the
// arguments to asmand, which typically follows the call to asmvex.
// The final two arguments are the VEX prefix (see encoding above)
// and the opcode byte.
// For details about vex prefix see:
// https://en.wikipedia.org/wiki/VEX_prefix#Technical_description
func (ab *AsmBuf) asmvex(pstate *PackageState, ctxt *obj.Link, rm, v, r *obj.Addr, vex, opcode uint8) {
	ab.vexflag = true
	rexR := 0
	if r != nil {
		rexR = pstate.regrex[r.Reg] & Rxr
	}
	rexB := 0
	rexX := 0
	if rm != nil {
		rexB = pstate.regrex[rm.Reg] & Rxb
		rexX = pstate.regrex[rm.Index] & Rxx
	}
	vexM := (vex >> 3) & 0x7
	vexWLP := vex & 0x87
	vexV := byte(0)
	if v != nil {
		vexV = byte(pstate.reg[v.Reg]|(pstate.regrex[v.Reg]&Rxr)<<1) & 0xF
	}
	vexV ^= 0xF
	if vexM == 1 && (rexX|rexB) == 0 && vex&vexW1 == 0 {
		// Can use 2-byte encoding.
		ab.Put2(0xc5, byte(rexR<<5)^0x80|vexV<<3|vexWLP)
	} else {
		// Must use 3-byte encoding.
		ab.Put3(0xc4,
			(byte(rexR|rexX|rexB)<<5)^0xE0|vexM,
			vexV<<3|vexWLP,
		)
	}
	ab.Put1(opcode)
}

// regIndex returns register index that fits in 5 bits.
//
//	R         : 3 bit | legacy instructions     | N/A
//	[R/V]EX.R : 1 bit | REX / VEX extension bit | Rxr
//	EVEX.R    : 1 bit | EVEX extension bit      | RxrEvex
//
// Examples:
//	REG_Z30 => 30
//	REG_X15 => 15
//	REG_R9  => 9
//	REG_AX  => 0
//
func (pstate *PackageState) regIndex(r int16) int {
	lower3bits := pstate.reg[r]
	high4bit := pstate.regrex[r] & Rxr << 1
	high5bit := pstate.regrex[r] & RxrEvex << 0
	return lower3bits | high4bit | high5bit
}

// avx2gatherValid reports whether p satisfies AVX2 gather constraints.
// Reports errors via ctxt.
func (pstate *PackageState) avx2gatherValid(ctxt *obj.Link, p *obj.Prog) bool {
	// If any pair of the index, mask, or destination registers
	// are the same, illegal instruction trap (#UD) is triggered.
	index := pstate.regIndex(p.GetFrom3().Index)
	mask := pstate.regIndex(p.From.Reg)
	dest := pstate.regIndex(p.To.Reg)
	if dest == mask || dest == index || mask == index {
		ctxt.Diag("mask, index, and destination registers should be distinct: %v", p)
		return false
	}

	return true
}

// avx512gatherValid reports whether p satisfies AVX512 gather constraints.
// Reports errors via ctxt.
func (pstate *PackageState) avx512gatherValid(ctxt *obj.Link, p *obj.Prog) bool {
	// Illegal instruction trap (#UD) is triggered if the destination vector
	// register is the same as index vector in VSIB.
	index := pstate.regIndex(p.From.Index)
	dest := pstate.regIndex(p.To.Reg)
	if dest == index {
		ctxt.Diag("index and destination registers should be distinct: %v", p)
		return false
	}

	return true
}

func (ab *AsmBuf) doasm(pstate *PackageState, ctxt *obj.Link, cursym *obj.LSym, p *obj.Prog) {
	o := pstate.opindex[p.As&obj.AMask]

	if o == nil {
		ctxt.Diag("asmins: missing op %v", p)
		return
	}

	if pre := pstate.prefixof(ctxt, &p.From); pre != 0 {
		ab.Put1(byte(pre))
	}
	if pre := pstate.prefixof(ctxt, &p.To); pre != 0 {
		ab.Put1(byte(pre))
	}

	// Checks to warn about instruction/arguments combinations that
	// will unconditionally trigger illegal instruction trap (#UD).
	switch p.As {
	case AVGATHERDPD,
		AVGATHERQPD,
		AVGATHERDPS,
		AVGATHERQPS,
		AVPGATHERDD,
		AVPGATHERQD,
		AVPGATHERDQ,
		AVPGATHERQQ:
		// AVX512 gather requires explicit K mask.
		if p.GetFrom3().Reg >= REG_K0 && p.GetFrom3().Reg <= REG_K7 {
			if !pstate.avx512gatherValid(ctxt, p) {
				return
			}
		} else {
			if !pstate.avx2gatherValid(ctxt, p) {
				return
			}
		}
	}

	if p.Ft == 0 {
		p.Ft = uint8(pstate.oclass(ctxt, p, &p.From))
	}
	if p.Tt == 0 {
		p.Tt = uint8(pstate.oclass(ctxt, p, &p.To))
	}

	ft := int(p.Ft) * Ymax
	var f3t int
	tt := int(p.Tt) * Ymax

	xo := obj.Bool2int(o.op[0] == 0x0f)
	z := 0
	var a *obj.Addr
	var l int
	var op int
	var q *obj.Prog
	var r *obj.Reloc
	var rel obj.Reloc
	var v int64

	args := make([]int, 0, argListMax)
	if ft != Ynone*Ymax {
		args = append(args, ft)
	}
	for i := range p.RestArgs {
		args = append(args, pstate.oclass(ctxt, p, &p.RestArgs[i])*Ymax)
	}
	if tt != Ynone*Ymax {
		args = append(args, tt)
	}

	for _, yt := range o.ytab {
		// ytab matching is purely args-based,
		// but AVX512 suffixes like "Z" or "RU_SAE" will
		// add EVEX-only filter that will reject non-EVEX matches.
		//
		// Consider "VADDPD.BCST 2032(DX), X0, X0".
		// Without this rule, operands will lead to VEX-encoded form
		// and produce "c5b15813" encoding.
		if !yt.match(pstate, args) {
			// "xo" is always zero for VEX/EVEX encoded insts.
			z += int(yt.zoffset) + xo
		} else {
			if p.Scond != 0 && !evexZcase(yt.zcase) {
				// Do not signal error and continue to search
				// for matching EVEX-encoded form.
				z += int(yt.zoffset)
				continue
			}

			switch o.prefix {
			case Px1: // first option valid only in 32-bit mode
				if ctxt.Arch.Family == sys.AMD64 && z == 0 {
					z += int(yt.zoffset) + xo
					continue
				}
			case Pq: // 16 bit escape and opcode escape
				ab.Put2(Pe, Pm)

			case Pq3: // 16 bit escape and opcode escape + REX.W
				ab.rexflag |= Pw
				ab.Put2(Pe, Pm)

			case Pq4: // 66 0F 38
				ab.Put3(0x66, 0x0F, 0x38)

			case Pq4w: // 66 0F 38 + REX.W
				ab.rexflag |= Pw
				ab.Put3(0x66, 0x0F, 0x38)

			case Pq5: // F3 0F 38
				ab.Put3(0xF3, 0x0F, 0x38)

			case Pq5w: //  F3 0F 38 + REX.W
				ab.rexflag |= Pw
				ab.Put3(0xF3, 0x0F, 0x38)

			case Pf2, // xmm opcode escape
				Pf3:
				ab.Put2(o.prefix, Pm)

			case Pef3:
				ab.Put3(Pe, Pf3, Pm)

			case Pfw: // xmm opcode escape + REX.W
				ab.rexflag |= Pw
				ab.Put2(Pf3, Pm)

			case Pm: // opcode escape
				ab.Put1(Pm)

			case Pe: // 16 bit escape
				ab.Put1(Pe)

			case Pw: // 64-bit escape
				if ctxt.Arch.Family != sys.AMD64 {
					ctxt.Diag("asmins: illegal 64: %v", p)
				}
				ab.rexflag |= Pw

			case Pw8: // 64-bit escape if z >= 8
				if z >= 8 {
					if ctxt.Arch.Family != sys.AMD64 {
						ctxt.Diag("asmins: illegal 64: %v", p)
					}
					ab.rexflag |= Pw
				}

			case Pb: // botch
				if ctxt.Arch.Family != sys.AMD64 && (isbadbyte(&p.From) || isbadbyte(&p.To)) {
					goto bad
				}
				// NOTE(rsc): This is probably safe to do always,
				// but when enabled it chooses different encodings
				// than the old cmd/internal/obj/i386 code did,
				// which breaks our "same bits out" checks.
				// In particular, CMPB AX, $0 encodes as 80 f8 00
				// in the original obj/i386, and it would encode
				// (using a valid, shorter form) as 3c 00 if we enabled
				// the call to bytereg here.
				if ctxt.Arch.Family == sys.AMD64 {
					bytereg(&p.From, &p.Ft)
					bytereg(&p.To, &p.Tt)
				}

			case P32: // 32 bit but illegal if 64-bit mode
				if ctxt.Arch.Family == sys.AMD64 {
					ctxt.Diag("asmins: illegal in 64-bit mode: %v", p)
				}

			case Py: // 64-bit only, no prefix
				if ctxt.Arch.Family != sys.AMD64 {
					ctxt.Diag("asmins: illegal in %d-bit mode: %v", ctxt.Arch.RegSize*8, p)
				}

			case Py1: // 64-bit only if z < 1, no prefix
				if z < 1 && ctxt.Arch.Family != sys.AMD64 {
					ctxt.Diag("asmins: illegal in %d-bit mode: %v", ctxt.Arch.RegSize*8, p)
				}

			case Py3: // 64-bit only if z < 3, no prefix
				if z < 3 && ctxt.Arch.Family != sys.AMD64 {
					ctxt.Diag("asmins: illegal in %d-bit mode: %v", ctxt.Arch.RegSize*8, p)
				}
			}

			if z >= len(o.op) {
				log.Fatalf("asmins bad table %v", p)
			}
			op = int(o.op[z])
			if op == 0x0f {
				ab.Put1(byte(op))
				z++
				op = int(o.op[z])
			}

			switch yt.zcase {
			default:
				ctxt.Diag("asmins: unknown z %d %v", yt.zcase, p)
				return

			case Zpseudo:
				break

			case Zlit:
				ab.PutOpBytesLit(z, &o.op)

			case Zlitr_m:
				ab.PutOpBytesLit(z, &o.op)
				ab.asmand(pstate, ctxt, cursym, p, &p.To, &p.From)

			case Zlitm_r:
				ab.PutOpBytesLit(z, &o.op)
				ab.asmand(pstate, ctxt, cursym, p, &p.From, &p.To)

			case Zlit_m_r:
				ab.PutOpBytesLit(z, &o.op)
				ab.asmand(pstate, ctxt, cursym, p, p.GetFrom3(), &p.To)

			case Zmb_r:
				bytereg(&p.From, &p.Ft)
				fallthrough

			case Zm_r:
				ab.Put1(byte(op))
				ab.asmand(pstate, ctxt, cursym, p, &p.From, &p.To)

			case Z_m_r:
				ab.Put1(byte(op))
				ab.asmand(pstate, ctxt, cursym, p, p.GetFrom3(), &p.To)

			case Zm2_r:
				ab.Put2(byte(op), o.op[z+1])
				ab.asmand(pstate, ctxt, cursym, p, &p.From, &p.To)

			case Zm_r_xm:
				ab.mediaop(ctxt, o, op, int(yt.zoffset), z)
				ab.asmand(pstate, ctxt, cursym, p, &p.From, &p.To)

			case Zm_r_xm_nr:
				ab.rexflag = 0
				ab.mediaop(ctxt, o, op, int(yt.zoffset), z)
				ab.asmand(pstate, ctxt, cursym, p, &p.From, &p.To)

			case Zm_r_i_xm:
				ab.mediaop(ctxt, o, op, int(yt.zoffset), z)
				ab.asmand(pstate, ctxt, cursym, p, &p.From, p.GetFrom3())
				ab.Put1(byte(p.To.Offset))

			case Zibm_r, Zibr_m:
				ab.PutOpBytesLit(z, &o.op)
				if yt.zcase == Zibr_m {
					ab.asmand(pstate, ctxt, cursym, p, &p.To, p.GetFrom3())
				} else {
					ab.asmand(pstate, ctxt, cursym, p, p.GetFrom3(), &p.To)
				}
				switch {
				default:
					ab.Put1(byte(p.From.Offset))
				case yt.args[0] == Yi32 && o.prefix == Pe:
					ab.PutInt16(int16(p.From.Offset))
				case yt.args[0] == Yi32:
					ab.PutInt32(int32(p.From.Offset))
				}

			case Zaut_r:
				ab.Put1(0x8d) // leal
				if p.From.Type != obj.TYPE_ADDR {
					ctxt.Diag("asmins: Zaut sb type ADDR")
				}
				p.From.Type = obj.TYPE_MEM
				ab.asmand(pstate, ctxt, cursym, p, &p.From, &p.To)
				p.From.Type = obj.TYPE_ADDR

			case Zm_o:
				ab.Put1(byte(op))
				ab.asmando(pstate, ctxt, cursym, p, &p.From, int(o.op[z+1]))

			case Zr_m:
				ab.Put1(byte(op))
				ab.asmand(pstate, ctxt, cursym, p, &p.To, &p.From)

			case Zvex:
				ab.asmvex(pstate, ctxt, &p.From, p.GetFrom3(), &p.To, o.op[z], o.op[z+1])

			case Zvex_rm_v_r:
				ab.asmvex(pstate, ctxt, &p.From, p.GetFrom3(), &p.To, o.op[z], o.op[z+1])
				ab.asmand(pstate, ctxt, cursym, p, &p.From, &p.To)

			case Zvex_rm_v_ro:
				ab.asmvex(pstate, ctxt, &p.From, p.GetFrom3(), &p.To, o.op[z], o.op[z+1])
				ab.asmando(pstate, ctxt, cursym, p, &p.From, int(o.op[z+2]))

			case Zvex_i_rm_vo:
				ab.asmvex(pstate, ctxt, p.GetFrom3(), &p.To, nil, o.op[z], o.op[z+1])
				ab.asmando(pstate, ctxt, cursym, p, p.GetFrom3(), int(o.op[z+2]))
				ab.Put1(byte(p.From.Offset))

			case Zvex_i_r_v:
				ab.asmvex(pstate, ctxt, p.GetFrom3(), &p.To, nil, o.op[z], o.op[z+1])
				regnum := byte(0x7)
				if p.GetFrom3().Reg >= REG_X0 && p.GetFrom3().Reg <= REG_X15 {
					regnum &= byte(p.GetFrom3().Reg - REG_X0)
				} else {
					regnum &= byte(p.GetFrom3().Reg - REG_Y0)
				}
				ab.Put1(o.op[z+2] | regnum)
				ab.Put1(byte(p.From.Offset))

			case Zvex_i_rm_v_r:
				imm, from, from3, to := unpackOps4(p)
				ab.asmvex(pstate, ctxt, from, from3, to, o.op[z], o.op[z+1])
				ab.asmand(pstate, ctxt, cursym, p, from, to)
				ab.Put1(byte(imm.Offset))

			case Zvex_i_rm_r:
				ab.asmvex(pstate, ctxt, p.GetFrom3(), nil, &p.To, o.op[z], o.op[z+1])
				ab.asmand(pstate, ctxt, cursym, p, p.GetFrom3(), &p.To)
				ab.Put1(byte(p.From.Offset))

			case Zvex_v_rm_r:
				ab.asmvex(pstate, ctxt, p.GetFrom3(), &p.From, &p.To, o.op[z], o.op[z+1])
				ab.asmand(pstate, ctxt, cursym, p, p.GetFrom3(), &p.To)

			case Zvex_r_v_rm:
				ab.asmvex(pstate, ctxt, &p.To, p.GetFrom3(), &p.From, o.op[z], o.op[z+1])
				ab.asmand(pstate, ctxt, cursym, p, &p.To, &p.From)

			case Zvex_rm_r_vo:
				ab.asmvex(pstate, ctxt, &p.From, &p.To, p.GetFrom3(), o.op[z], o.op[z+1])
				ab.asmando(pstate, ctxt, cursym, p, &p.From, int(o.op[z+2]))

			case Zvex_i_r_rm:
				ab.asmvex(pstate, ctxt, &p.To, nil, p.GetFrom3(), o.op[z], o.op[z+1])
				ab.asmand(pstate, ctxt, cursym, p, &p.To, p.GetFrom3())
				ab.Put1(byte(p.From.Offset))

			case Zvex_hr_rm_v_r:
				hr, from, from3, to := unpackOps4(p)
				ab.asmvex(pstate, ctxt, from, from3, to, o.op[z], o.op[z+1])
				ab.asmand(pstate, ctxt, cursym, p, from, to)
				ab.Put1(byte(pstate.regIndex(hr.Reg) << 4))

			case Zevex_k_rmo:
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, &p.To, nil, nil, &p.From)
				ab.asmando(pstate, ctxt, cursym, p, &p.To, int(o.op[z+3]))

			case Zevex_i_rm_vo:
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, p.GetFrom3(), &p.To, nil, nil)
				ab.asmando(pstate, ctxt, cursym, p, p.GetFrom3(), int(o.op[z+3]))
				ab.Put1(byte(p.From.Offset))

			case Zevex_i_rm_k_vo:
				imm, from, kmask, to := unpackOps4(p)
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, from, to, nil, kmask)
				ab.asmando(pstate, ctxt, cursym, p, from, int(o.op[z+3]))
				ab.Put1(byte(imm.Offset))

			case Zevex_i_r_rm:
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, &p.To, nil, p.GetFrom3(), nil)
				ab.asmand(pstate, ctxt, cursym, p, &p.To, p.GetFrom3())
				ab.Put1(byte(p.From.Offset))

			case Zevex_i_r_k_rm:
				imm, from, kmask, to := unpackOps4(p)
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, to, nil, from, kmask)
				ab.asmand(pstate, ctxt, cursym, p, to, from)
				ab.Put1(byte(imm.Offset))

			case Zevex_i_rm_r:
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, p.GetFrom3(), nil, &p.To, nil)
				ab.asmand(pstate, ctxt, cursym, p, p.GetFrom3(), &p.To)
				ab.Put1(byte(p.From.Offset))

			case Zevex_i_rm_k_r:
				imm, from, kmask, to := unpackOps4(p)
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, from, nil, to, kmask)
				ab.asmand(pstate, ctxt, cursym, p, from, to)
				ab.Put1(byte(imm.Offset))

			case Zevex_i_rm_v_r:
				imm, from, from3, to := unpackOps4(p)
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, from, from3, to, nil)
				ab.asmand(pstate, ctxt, cursym, p, from, to)
				ab.Put1(byte(imm.Offset))

			case Zevex_i_rm_v_k_r:
				imm, from, from3, kmask, to := unpackOps5(p)
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, from, from3, to, kmask)
				ab.asmand(pstate, ctxt, cursym, p, from, to)
				ab.Put1(byte(imm.Offset))

			case Zevex_r_v_rm:
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, &p.To, p.GetFrom3(), &p.From, nil)
				ab.asmand(pstate, ctxt, cursym, p, &p.To, &p.From)

			case Zevex_rm_v_r:
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, &p.From, p.GetFrom3(), &p.To, nil)
				ab.asmand(pstate, ctxt, cursym, p, &p.From, &p.To)

			case Zevex_rm_k_r:
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, &p.From, nil, &p.To, p.GetFrom3())
				ab.asmand(pstate, ctxt, cursym, p, &p.From, &p.To)

			case Zevex_r_k_rm:
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, &p.To, nil, &p.From, p.GetFrom3())
				ab.asmand(pstate, ctxt, cursym, p, &p.To, &p.From)

			case Zevex_rm_v_k_r:
				from, from3, kmask, to := unpackOps4(p)
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, from, from3, to, kmask)
				ab.asmand(pstate, ctxt, cursym, p, from, to)

			case Zevex_r_v_k_rm:
				from, from3, kmask, to := unpackOps4(p)
				ab.evex = newEVEXBits(z, &o.op)
				ab.asmevex(pstate, ctxt, p, to, from3, from, kmask)
				ab.asmand(pstate, ctxt, cursym, p, to, from)

			case Zr_m_xm:
				ab.mediaop(ctxt, o, op, int(yt.zoffset), z)
				ab.asmand(pstate, ctxt, cursym, p, &p.To, &p.From)

			case Zr_m_xm_nr:
				ab.rexflag = 0
				ab.mediaop(ctxt, o, op, int(yt.zoffset), z)
				ab.asmand(pstate, ctxt, cursym, p, &p.To, &p.From)

			case Zo_m:
				ab.Put1(byte(op))
				ab.asmando(pstate, ctxt, cursym, p, &p.To, int(o.op[z+1]))

			case Zcallindreg:
				r = obj.Addrel(cursym)
				r.Off = int32(p.Pc)
				r.Type = objabi.R_CALLIND
				r.Siz = 0
				fallthrough

			case Zo_m64:
				ab.Put1(byte(op))
				ab.asmandsz(pstate, ctxt, cursym, p, &p.To, int(o.op[z+1]), 0, 1)

			case Zm_ibo:
				ab.Put1(byte(op))
				ab.asmando(pstate, ctxt, cursym, p, &p.From, int(o.op[z+1]))
				ab.Put1(byte(pstate.vaddr(ctxt, p, &p.To, nil)))

			case Zibo_m:
				ab.Put1(byte(op))
				ab.asmando(pstate, ctxt, cursym, p, &p.To, int(o.op[z+1]))
				ab.Put1(byte(pstate.vaddr(ctxt, p, &p.From, nil)))

			case Zibo_m_xm:
				z = ab.mediaop(ctxt, o, op, int(yt.zoffset), z)
				ab.asmando(pstate, ctxt, cursym, p, &p.To, int(o.op[z+1]))
				ab.Put1(byte(pstate.vaddr(ctxt, p, &p.From, nil)))

			case Z_ib, Zib_:
				if yt.zcase == Zib_ {
					a = &p.From
				} else {
					a = &p.To
				}
				ab.Put1(byte(op))
				if p.As == AXABORT {
					ab.Put1(o.op[z+1])
				}
				ab.Put1(byte(pstate.vaddr(ctxt, p, a, nil)))

			case Zib_rp:
				ab.rexflag |= pstate.regrex[p.To.Reg] & (Rxb | 0x40)
				ab.Put2(byte(op+pstate.reg[p.To.Reg]), byte(pstate.vaddr(ctxt, p, &p.From, nil)))

			case Zil_rp:
				ab.rexflag |= pstate.regrex[p.To.Reg] & Rxb
				ab.Put1(byte(op + pstate.reg[p.To.Reg]))
				if o.prefix == Pe {
					v = pstate.vaddr(ctxt, p, &p.From, nil)
					ab.PutInt16(int16(v))
				} else {
					ab.relput4(pstate, ctxt, cursym, p, &p.From)
				}

			case Zo_iw:
				ab.Put1(byte(op))
				if p.From.Type != obj.TYPE_NONE {
					v = pstate.vaddr(ctxt, p, &p.From, nil)
					ab.PutInt16(int16(v))
				}

			case Ziq_rp:
				v = pstate.vaddr(ctxt, p, &p.From, &rel)
				l = int(v >> 32)
				if l == 0 && rel.Siz != 8 {
					ab.rexflag &^= (0x40 | Rxw)

					ab.rexflag |= pstate.regrex[p.To.Reg] & Rxb
					ab.Put1(byte(0xb8 + pstate.reg[p.To.Reg]))
					if rel.Type != 0 {
						r = obj.Addrel(cursym)
						*r = rel
						r.Off = int32(p.Pc + int64(ab.Len()))
					}

					ab.PutInt32(int32(v))
				} else if l == -1 && uint64(v)&(uint64(1)<<31) != 0 { // sign extend
					ab.Put1(0xc7)
					ab.asmando(pstate, ctxt, cursym, p, &p.To, 0)

					ab.PutInt32(int32(v)) // need all 8
				} else {
					ab.rexflag |= pstate.regrex[p.To.Reg] & Rxb
					ab.Put1(byte(op + pstate.reg[p.To.Reg]))
					if rel.Type != 0 {
						r = obj.Addrel(cursym)
						*r = rel
						r.Off = int32(p.Pc + int64(ab.Len()))
					}

					ab.PutInt64(v)
				}

			case Zib_rr:
				ab.Put1(byte(op))
				ab.asmand(pstate, ctxt, cursym, p, &p.To, &p.To)
				ab.Put1(byte(pstate.vaddr(ctxt, p, &p.From, nil)))

			case Z_il, Zil_:
				if yt.zcase == Zil_ {
					a = &p.From
				} else {
					a = &p.To
				}
				ab.Put1(byte(op))
				if o.prefix == Pe {
					v = pstate.vaddr(ctxt, p, a, nil)
					ab.PutInt16(int16(v))
				} else {
					ab.relput4(pstate, ctxt, cursym, p, a)
				}

			case Zm_ilo, Zilo_m:
				ab.Put1(byte(op))
				if yt.zcase == Zilo_m {
					a = &p.From
					ab.asmando(pstate, ctxt, cursym, p, &p.To, int(o.op[z+1]))
				} else {
					a = &p.To
					ab.asmando(pstate, ctxt, cursym, p, &p.From, int(o.op[z+1]))
				}

				if o.prefix == Pe {
					v = pstate.vaddr(ctxt, p, a, nil)
					ab.PutInt16(int16(v))
				} else {
					ab.relput4(pstate, ctxt, cursym, p, a)
				}

			case Zil_rr:
				ab.Put1(byte(op))
				ab.asmand(pstate, ctxt, cursym, p, &p.To, &p.To)
				if o.prefix == Pe {
					v = pstate.vaddr(ctxt, p, &p.From, nil)
					ab.PutInt16(int16(v))
				} else {
					ab.relput4(pstate, ctxt, cursym, p, &p.From)
				}

			case Z_rp:
				ab.rexflag |= pstate.regrex[p.To.Reg] & (Rxb | 0x40)
				ab.Put1(byte(op + pstate.reg[p.To.Reg]))

			case Zrp_:
				ab.rexflag |= pstate.regrex[p.From.Reg] & (Rxb | 0x40)
				ab.Put1(byte(op + pstate.reg[p.From.Reg]))

			case Zcallcon, Zjmpcon:
				if yt.zcase == Zcallcon {
					ab.Put1(byte(op))
				} else {
					ab.Put1(o.op[z+1])
				}
				r = obj.Addrel(cursym)
				r.Off = int32(p.Pc + int64(ab.Len()))
				r.Type = objabi.R_PCREL
				r.Siz = 4
				r.Add = p.To.Offset
				ab.PutInt32(0)

			case Zcallind:
				ab.Put2(byte(op), o.op[z+1])
				r = obj.Addrel(cursym)
				r.Off = int32(p.Pc + int64(ab.Len()))
				if ctxt.Arch.Family == sys.AMD64 {
					r.Type = objabi.R_PCREL
				} else {
					r.Type = objabi.R_ADDR
				}
				r.Siz = 4
				r.Add = p.To.Offset
				r.Sym = p.To.Sym
				ab.PutInt32(0)

			case Zcall, Zcallduff:
				if p.To.Sym == nil {
					ctxt.Diag("call without target")
					ctxt.DiagFlush()
					log.Fatalf("bad code")
				}

				if yt.zcase == Zcallduff && ctxt.Flag_dynlink {
					ctxt.Diag("directly calling duff when dynamically linking Go")
				}

				if ctxt.Framepointer_enabled && yt.zcase == Zcallduff && ctxt.Arch.Family == sys.AMD64 {
					// Maintain BP around call, since duffcopy/duffzero can't do it
					// (the call jumps into the middle of the function).
					// This makes it possible to see call sites for duffcopy/duffzero in
					// BP-based profiling tools like Linux perf (which is the
					// whole point of obj.Framepointer_enabled).
					// MOVQ BP, -16(SP)
					// LEAQ -16(SP), BP
					ab.Put(pstate.bpduff1)
				}
				ab.Put1(byte(op))
				r = obj.Addrel(cursym)
				r.Off = int32(p.Pc + int64(ab.Len()))
				r.Sym = p.To.Sym
				r.Add = p.To.Offset
				r.Type = objabi.R_CALL
				r.Siz = 4
				ab.PutInt32(0)

				if ctxt.Framepointer_enabled && yt.zcase == Zcallduff && ctxt.Arch.Family == sys.AMD64 {
					// Pop BP pushed above.
					// MOVQ 0(BP), BP
					ab.Put(pstate.bpduff2)
				}

			// TODO: jump across functions needs reloc
			case Zbr, Zjmp, Zloop:
				if p.As == AXBEGIN {
					ab.Put1(byte(op))
				}
				if p.To.Sym != nil {
					if yt.zcase != Zjmp {
						ctxt.Diag("branch to ATEXT")
						ctxt.DiagFlush()
						log.Fatalf("bad code")
					}

					ab.Put1(o.op[z+1])
					r = obj.Addrel(cursym)
					r.Off = int32(p.Pc + int64(ab.Len()))
					r.Sym = p.To.Sym
					r.Type = objabi.R_PCREL
					r.Siz = 4
					ab.PutInt32(0)
					break
				}

				// Assumes q is in this function.
				// TODO: Check in input, preserve in brchain.

				// Fill in backward jump now.
				q = p.Pcond

				if q == nil {
					ctxt.Diag("jmp/branch/loop without target")
					ctxt.DiagFlush()
					log.Fatalf("bad code")
				}

				if p.Back&branchBackwards != 0 {
					v = q.Pc - (p.Pc + 2)
					if v >= -128 && p.As != AXBEGIN {
						if p.As == AJCXZL {
							ab.Put1(0x67)
						}
						ab.Put2(byte(op), byte(v))
					} else if yt.zcase == Zloop {
						ctxt.Diag("loop too far: %v", p)
					} else {
						v -= 5 - 2
						if p.As == AXBEGIN {
							v--
						}
						if yt.zcase == Zbr {
							ab.Put1(0x0f)
							v--
						}

						ab.Put1(o.op[z+1])
						ab.PutInt32(int32(v))
					}

					break
				}

				// Annotate target; will fill in later.
				p.Forwd = q.Rel

				q.Rel = p
				if p.Back&branchShort != 0 && p.As != AXBEGIN {
					if p.As == AJCXZL {
						ab.Put1(0x67)
					}
					ab.Put2(byte(op), 0)
				} else if yt.zcase == Zloop {
					ctxt.Diag("loop too far: %v", p)
				} else {
					if yt.zcase == Zbr {
						ab.Put1(0x0f)
					}
					ab.Put1(o.op[z+1])
					ab.PutInt32(0)
				}

			case Zbyte:
				v = pstate.vaddr(ctxt, p, &p.From, &rel)
				if rel.Siz != 0 {
					rel.Siz = uint8(op)
					r = obj.Addrel(cursym)
					*r = rel
					r.Off = int32(p.Pc + int64(ab.Len()))
				}

				ab.Put1(byte(v))
				if op > 1 {
					ab.Put1(byte(v >> 8))
					if op > 2 {
						ab.PutInt16(int16(v >> 16))
						if op > 4 {
							ab.PutInt32(int32(v >> 32))
						}
					}
				}
			}

			return
		}
	}
	f3t = Ynone * Ymax
	if p.GetFrom3() != nil {
		f3t = pstate.oclass(ctxt, p, p.GetFrom3()) * Ymax
	}
	for mo := pstate.ymovtab; mo[0].as != 0; mo = mo[1:] {
		var pp obj.Prog
		var t []byte
		if p.As == mo[0].as {
			if pstate.ycover[ft+int(mo[0].ft)] != 0 && pstate.ycover[f3t+int(mo[0].f3t)] != 0 && pstate.ycover[tt+int(mo[0].tt)] != 0 {
				t = mo[0].op[:]
				switch mo[0].code {
				default:
					ctxt.Diag("asmins: unknown mov %d %v", mo[0].code, p)

				case movLit:
					for z = 0; t[z] != 0; z++ {
						ab.Put1(t[z])
					}

				case movRegMem:
					ab.Put1(t[0])
					ab.asmando(pstate, ctxt, cursym, p, &p.To, int(t[1]))

				case movMemReg:
					ab.Put1(t[0])
					ab.asmando(pstate, ctxt, cursym, p, &p.From, int(t[1]))

				case movRegMem2op: // r,m - 2op
					ab.Put2(t[0], t[1])
					ab.asmando(pstate, ctxt, cursym, p, &p.To, int(t[2]))
					ab.rexflag |= pstate.regrex[p.From.Reg] & (Rxr | 0x40)

				case movMemReg2op:
					ab.Put2(t[0], t[1])
					ab.asmando(pstate, ctxt, cursym, p, &p.From, int(t[2]))
					ab.rexflag |= pstate.regrex[p.To.Reg] & (Rxr | 0x40)

				case movFullPtr:
					if t[0] != 0 {
						ab.Put1(t[0])
					}
					switch p.To.Index {
					default:
						goto bad

					case REG_DS:
						ab.Put1(0xc5)

					case REG_SS:
						ab.Put2(0x0f, 0xb2)

					case REG_ES:
						ab.Put1(0xc4)

					case REG_FS:
						ab.Put2(0x0f, 0xb4)

					case REG_GS:
						ab.Put2(0x0f, 0xb5)
					}

					ab.asmand(pstate, ctxt, cursym, p, &p.From, &p.To)

				case movDoubleShift:
					if t[0] == Pw {
						if ctxt.Arch.Family != sys.AMD64 {
							ctxt.Diag("asmins: illegal 64: %v", p)
						}
						ab.rexflag |= Pw
						t = t[1:]
					} else if t[0] == Pe {
						ab.Put1(Pe)
						t = t[1:]
					}

					switch p.From.Type {
					default:
						goto bad

					case obj.TYPE_CONST:
						ab.Put2(0x0f, t[0])
						ab.asmandsz(pstate, ctxt, cursym, p, &p.To, pstate.reg[p.GetFrom3().Reg], pstate.regrex[p.GetFrom3().Reg], 0)
						ab.Put1(byte(p.From.Offset))

					case obj.TYPE_REG:
						switch p.From.Reg {
						default:
							goto bad

						case REG_CL, REG_CX:
							ab.Put2(0x0f, t[1])
							ab.asmandsz(pstate, ctxt, cursym, p, &p.To, pstate.reg[p.GetFrom3().Reg], pstate.regrex[p.GetFrom3().Reg], 0)
						}
					}

				// NOTE: The systems listed here are the ones that use the "TLS initial exec" model,
				// where you load the TLS base register into a register and then index off that
				// register to access the actual TLS variables. Systems that allow direct TLS access
				// are handled in prefixof above and should not be listed here.
				case movTLSReg:
					if ctxt.Arch.Family == sys.AMD64 && p.As != AMOVQ || ctxt.Arch.Family == sys.I386 && p.As != AMOVL {
						ctxt.Diag("invalid load of TLS: %v", p)
					}

					if ctxt.Arch.Family == sys.I386 {
						// NOTE: The systems listed here are the ones that use the "TLS initial exec" model,
						// where you load the TLS base register into a register and then index off that
						// register to access the actual TLS variables. Systems that allow direct TLS access
						// are handled in prefixof above and should not be listed here.
						switch ctxt.Headtype {
						default:
							log.Fatalf("unknown TLS base location for %v", ctxt.Headtype)

						case objabi.Hlinux,
							objabi.Hnacl, objabi.Hfreebsd:
							if ctxt.Flag_shared {
								// Note that this is not generating the same insns as the other cases.
								//     MOV TLS, dst
								// becomes
								//     call __x86.get_pc_thunk.dst
								//     movl (gotpc + g@gotntpoff)(dst), dst
								// which is encoded as
								//     call __x86.get_pc_thunk.dst
								//     movq 0(dst), dst
								// and R_CALL & R_TLS_IE relocs. This all assumes the only tls variable we access
								// is g, which we can't check here, but will when we assemble the second
								// instruction.
								dst := p.To.Reg
								ab.Put1(0xe8)
								r = obj.Addrel(cursym)
								r.Off = int32(p.Pc + int64(ab.Len()))
								r.Type = objabi.R_CALL
								r.Siz = 4
								r.Sym = ctxt.Lookup("__x86.get_pc_thunk." + strings.ToLower(pstate.rconv(int(dst))))
								ab.PutInt32(0)

								ab.Put2(0x8B, byte(2<<6|pstate.reg[dst]|(pstate.reg[dst]<<3)))
								r = obj.Addrel(cursym)
								r.Off = int32(p.Pc + int64(ab.Len()))
								r.Type = objabi.R_TLS_IE
								r.Siz = 4
								r.Add = 2
								ab.PutInt32(0)
							} else {
								// ELF TLS base is 0(GS).
								pp.From = p.From

								pp.From.Type = obj.TYPE_MEM
								pp.From.Reg = REG_GS
								pp.From.Offset = 0
								pp.From.Index = REG_NONE
								pp.From.Scale = 0
								ab.Put2(0x65, // GS
									0x8B)
								ab.asmand(pstate, ctxt, cursym, p, &pp.From, &p.To)
							}
						case objabi.Hplan9:
							pp.From = obj.Addr{}
							pp.From.Type = obj.TYPE_MEM
							pp.From.Name = obj.NAME_EXTERN
							pp.From.Sym = pstate.plan9privates
							pp.From.Offset = 0
							pp.From.Index = REG_NONE
							ab.Put1(0x8B)
							ab.asmand(pstate, ctxt, cursym, p, &pp.From, &p.To)

						case objabi.Hwindows:
							// Windows TLS base is always 0x14(FS).
							pp.From = p.From

							pp.From.Type = obj.TYPE_MEM
							pp.From.Reg = REG_FS
							pp.From.Offset = 0x14
							pp.From.Index = REG_NONE
							pp.From.Scale = 0
							ab.Put2(0x64, // FS
								0x8B)
							ab.asmand(pstate, ctxt, cursym, p, &pp.From, &p.To)
						}
						break
					}

					switch ctxt.Headtype {
					default:
						log.Fatalf("unknown TLS base location for %v", ctxt.Headtype)

					case objabi.Hlinux, objabi.Hfreebsd:
						if !ctxt.Flag_shared {
							log.Fatalf("unknown TLS base location for linux/freebsd without -shared")
						}
						// Note that this is not generating the same insn as the other cases.
						//     MOV TLS, R_to
						// becomes
						//     movq g@gottpoff(%rip), R_to
						// which is encoded as
						//     movq 0(%rip), R_to
						// and a R_TLS_IE reloc. This all assumes the only tls variable we access
						// is g, which we can't check here, but will when we assemble the second
						// instruction.
						ab.rexflag = Pw | (pstate.regrex[p.To.Reg] & Rxr)

						ab.Put2(0x8B, byte(0x05|(pstate.reg[p.To.Reg]<<3)))
						r = obj.Addrel(cursym)
						r.Off = int32(p.Pc + int64(ab.Len()))
						r.Type = objabi.R_TLS_IE
						r.Siz = 4
						r.Add = -4
						ab.PutInt32(0)

					case objabi.Hplan9:
						pp.From = obj.Addr{}
						pp.From.Type = obj.TYPE_MEM
						pp.From.Name = obj.NAME_EXTERN
						pp.From.Sym = pstate.plan9privates
						pp.From.Offset = 0
						pp.From.Index = REG_NONE
						ab.rexflag |= Pw
						ab.Put1(0x8B)
						ab.asmand(pstate, ctxt, cursym, p, &pp.From, &p.To)

					case objabi.Hsolaris: // TODO(rsc): Delete Hsolaris from list. Should not use this code. See progedit in obj6.c.
						// TLS base is 0(FS).
						pp.From = p.From

						pp.From.Type = obj.TYPE_MEM
						pp.From.Name = obj.NAME_NONE
						pp.From.Reg = REG_NONE
						pp.From.Offset = 0
						pp.From.Index = REG_NONE
						pp.From.Scale = 0
						ab.rexflag |= Pw
						ab.Put2(0x64, // FS
							0x8B)
						ab.asmand(pstate, ctxt, cursym, p, &pp.From, &p.To)

					case objabi.Hwindows:
						// Windows TLS base is always 0x28(GS).
						pp.From = p.From

						pp.From.Type = obj.TYPE_MEM
						pp.From.Name = obj.NAME_NONE
						pp.From.Reg = REG_GS
						pp.From.Offset = 0x28
						pp.From.Index = REG_NONE
						pp.From.Scale = 0
						ab.rexflag |= Pw
						ab.Put2(0x65, // GS
							0x8B)
						ab.asmand(pstate, ctxt, cursym, p, &pp.From, &p.To)
					}
				}
				return
			}
		}
	}
	goto bad

bad:
	if ctxt.Arch.Family != sys.AMD64 {
		// here, the assembly has failed.
		// if its a byte instruction that has
		// unaddressable registers, try to
		// exchange registers and reissue the
		// instruction with the operands renamed.
		pp := *p

		unbytereg(&pp.From, &pp.Ft)
		unbytereg(&pp.To, &pp.Tt)

		z := int(p.From.Reg)
		if p.From.Type == obj.TYPE_REG && z >= REG_BP && z <= REG_DI {
			// TODO(rsc): Use this code for x86-64 too. It has bug fixes not present in the amd64 code base.
			// For now, different to keep bit-for-bit compatibility.
			if ctxt.Arch.Family == sys.I386 {
				breg := byteswapreg(ctxt, &p.To)
				if breg != REG_AX {
					ab.Put1(0x87) // xchg lhs,bx
					ab.asmando(pstate, ctxt, cursym, p, &p.From, pstate.reg[breg])
					pstate.subreg(&pp, z, breg)
					ab.doasm(pstate, ctxt, cursym, &pp)
					ab.Put1(0x87) // xchg lhs,bx
					ab.asmando(pstate, ctxt, cursym, p, &p.From, pstate.reg[breg])
				} else {
					ab.Put1(byte(0x90 + pstate.reg[z])) // xchg lsh,ax
					pstate.subreg(&pp, z, REG_AX)
					ab.doasm(pstate, ctxt, cursym, &pp)
					ab.Put1(byte(0x90 + pstate.reg[z])) // xchg lsh,ax
				}
				return
			}

			if isax(&p.To) || p.To.Type == obj.TYPE_NONE {
				// We certainly don't want to exchange
				// with AX if the op is MUL or DIV.
				ab.Put1(0x87) // xchg lhs,bx
				ab.asmando(pstate, ctxt, cursym, p, &p.From, pstate.reg[REG_BX])
				pstate.subreg(&pp, z, REG_BX)
				ab.doasm(pstate, ctxt, cursym, &pp)
				ab.Put1(0x87) // xchg lhs,bx
				ab.asmando(pstate, ctxt, cursym, p, &p.From, pstate.reg[REG_BX])
			} else {
				ab.Put1(byte(0x90 + pstate.reg[z])) // xchg lsh,ax
				pstate.subreg(&pp, z, REG_AX)
				ab.doasm(pstate, ctxt, cursym, &pp)
				ab.Put1(byte(0x90 + pstate.reg[z])) // xchg lsh,ax
			}
			return
		}

		z = int(p.To.Reg)
		if p.To.Type == obj.TYPE_REG && z >= REG_BP && z <= REG_DI {
			// TODO(rsc): Use this code for x86-64 too. It has bug fixes not present in the amd64 code base.
			// For now, different to keep bit-for-bit compatibility.
			if ctxt.Arch.Family == sys.I386 {
				breg := byteswapreg(ctxt, &p.From)
				if breg != REG_AX {
					ab.Put1(0x87) //xchg rhs,bx
					ab.asmando(pstate, ctxt, cursym, p, &p.To, pstate.reg[breg])
					pstate.subreg(&pp, z, breg)
					ab.doasm(pstate, ctxt, cursym, &pp)
					ab.Put1(0x87) // xchg rhs,bx
					ab.asmando(pstate, ctxt, cursym, p, &p.To, pstate.reg[breg])
				} else {
					ab.Put1(byte(0x90 + pstate.reg[z])) // xchg rsh,ax
					pstate.subreg(&pp, z, REG_AX)
					ab.doasm(pstate, ctxt, cursym, &pp)
					ab.Put1(byte(0x90 + pstate.reg[z])) // xchg rsh,ax
				}
				return
			}

			if isax(&p.From) {
				ab.Put1(0x87) // xchg rhs,bx
				ab.asmando(pstate, ctxt, cursym, p, &p.To, pstate.reg[REG_BX])
				pstate.subreg(&pp, z, REG_BX)
				ab.doasm(pstate, ctxt, cursym, &pp)
				ab.Put1(0x87) // xchg rhs,bx
				ab.asmando(pstate, ctxt, cursym, p, &p.To, pstate.reg[REG_BX])
			} else {
				ab.Put1(byte(0x90 + pstate.reg[z])) // xchg rsh,ax
				pstate.subreg(&pp, z, REG_AX)
				ab.doasm(pstate, ctxt, cursym, &pp)
				ab.Put1(byte(0x90 + pstate.reg[z])) // xchg rsh,ax
			}
			return
		}
	}

	ctxt.Diag("invalid instruction: %v", p)
	//	ctxt.Diag("doasm: notfound ft=%d tt=%d %v %d %d", p.Ft, p.Tt, p, oclass(ctxt, p, &p.From), oclass(ctxt, p, &p.To))
}

// byteswapreg returns a byte-addressable register (AX, BX, CX, DX)
// which is not referenced in a.
// If a is empty, it returns BX to account for MULB-like instructions
// that might use DX and AX.
func byteswapreg(ctxt *obj.Link, a *obj.Addr) int {
	cana, canb, canc, cand := true, true, true, true
	if a.Type == obj.TYPE_NONE {
		cana, cand = false, false
	}

	if a.Type == obj.TYPE_REG || ((a.Type == obj.TYPE_MEM || a.Type == obj.TYPE_ADDR) && a.Name == obj.NAME_NONE) {
		switch a.Reg {
		case REG_NONE:
			cana, cand = false, false
		case REG_AX, REG_AL, REG_AH:
			cana = false
		case REG_BX, REG_BL, REG_BH:
			canb = false
		case REG_CX, REG_CL, REG_CH:
			canc = false
		case REG_DX, REG_DL, REG_DH:
			cand = false
		}
	}

	if a.Type == obj.TYPE_MEM || a.Type == obj.TYPE_ADDR {
		switch a.Index {
		case REG_AX:
			cana = false
		case REG_BX:
			canb = false
		case REG_CX:
			canc = false
		case REG_DX:
			cand = false
		}
	}

	switch {
	case cana:
		return REG_AX
	case canb:
		return REG_BX
	case canc:
		return REG_CX
	case cand:
		return REG_DX
	default:
		ctxt.Diag("impossible byte register")
		ctxt.DiagFlush()
		log.Fatalf("bad code")
		return 0
	}
}

func isbadbyte(a *obj.Addr) bool {
	return a.Type == obj.TYPE_REG && (REG_BP <= a.Reg && a.Reg <= REG_DI || REG_BPB <= a.Reg && a.Reg <= REG_DIB)
}

func (ab *AsmBuf) nacltrunc(ctxt *obj.Link, reg int) {
	if reg >= REG_R8 {
		ab.Put1(0x45)
	}
	reg = (reg - REG_AX) & 7
	ab.Put2(0x89, byte(3<<6|reg<<3|reg))
}

func (ab *AsmBuf) asmins(pstate *PackageState, ctxt *obj.Link, cursym *obj.LSym, p *obj.Prog) {
	ab.Reset()

	if ctxt.Headtype == objabi.Hnacl && ctxt.Arch.Family == sys.I386 {
		switch p.As {
		case obj.ARET:
			ab.Put(pstate.naclret8)
			return

		case obj.ACALL,
			obj.AJMP:
			if p.To.Type == obj.TYPE_REG && REG_AX <= p.To.Reg && p.To.Reg <= REG_DI {
				ab.Put3(0x83, byte(0xe0|(p.To.Reg-REG_AX)), 0xe0)
			}

		case AINT:
			ab.Put1(0xf4)
			return
		}
	}

	if ctxt.Headtype == objabi.Hnacl && ctxt.Arch.Family == sys.AMD64 {
		if p.As == AREP {
			ab.rep = true
			return
		}

		if p.As == AREPN {
			ab.repn = true
			return
		}

		if p.As == ALOCK {
			ab.lock = true
			return
		}

		if p.As != ALEAQ && p.As != ALEAL {
			if p.From.Index != REG_NONE && p.From.Scale > 0 {
				ab.nacltrunc(ctxt, int(p.From.Index))
			}
			if p.To.Index != REG_NONE && p.To.Scale > 0 {
				ab.nacltrunc(ctxt, int(p.To.Index))
			}
		}

		switch p.As {
		case obj.ARET:
			ab.Put(pstate.naclret)
			return

		case obj.ACALL,
			obj.AJMP:
			if p.To.Type == obj.TYPE_REG && REG_AX <= p.To.Reg && p.To.Reg <= REG_DI {
				// ANDL $~31, reg
				ab.Put3(0x83, byte(0xe0|(p.To.Reg-REG_AX)), 0xe0)
				// ADDQ R15, reg
				ab.Put3(0x4c, 0x01, byte(0xf8|(p.To.Reg-REG_AX)))
			}

			if p.To.Type == obj.TYPE_REG && REG_R8 <= p.To.Reg && p.To.Reg <= REG_R15 {
				// ANDL $~31, reg
				ab.Put4(0x41, 0x83, byte(0xe0|(p.To.Reg-REG_R8)), 0xe0)
				// ADDQ R15, reg
				ab.Put3(0x4d, 0x01, byte(0xf8|(p.To.Reg-REG_R8)))
			}

		case AINT:
			ab.Put1(0xf4)
			return

		case ASCASB,
			ASCASW,
			ASCASL,
			ASCASQ,
			ASTOSB,
			ASTOSW,
			ASTOSL,
			ASTOSQ:
			ab.Put(pstate.naclstos)

		case AMOVSB, AMOVSW, AMOVSL, AMOVSQ:
			ab.Put(pstate.naclmovs)
		}

		if ab.rep {
			ab.Put1(0xf3)
			ab.rep = false
		}

		if ab.repn {
			ab.Put1(0xf2)
			ab.repn = false
		}

		if ab.lock {
			ab.Put1(0xf0)
			ab.lock = false
		}
	}

	ab.rexflag = 0
	ab.vexflag = false
	ab.evexflag = false
	mark := ab.Len()
	ab.doasm(pstate, ctxt, cursym, p)
	if ab.rexflag != 0 && !ab.vexflag && !ab.evexflag {
		// as befits the whole approach of the architecture,
		// the rex prefix must appear before the first opcode byte
		// (and thus after any 66/67/f2/f3/26/2e/3e prefix bytes, but
		// before the 0f opcode escape!), or it might be ignored.
		// note that the handbook often misleadingly shows 66/f2/f3 in `opcode'.
		if ctxt.Arch.Family != sys.AMD64 {
			ctxt.Diag("asmins: illegal in mode %d: %v (%d %d)", ctxt.Arch.RegSize*8, p, p.Ft, p.Tt)
		}
		n := ab.Len()
		var np int
		for np = mark; np < n; np++ {
			c := ab.At(np)
			if c != 0xf2 && c != 0xf3 && (c < 0x64 || c > 0x67) && c != 0x2e && c != 0x3e && c != 0x26 {
				break
			}
		}
		ab.Insert(np, byte(0x40|ab.rexflag))
	}

	n := ab.Len()
	for i := len(cursym.R) - 1; i >= 0; i-- {
		r := &cursym.R[i]
		if int64(r.Off) < p.Pc {
			break
		}
		if ab.rexflag != 0 && !ab.vexflag {
			r.Off++
		}
		if r.Type == objabi.R_PCREL {
			if ctxt.Arch.Family == sys.AMD64 || p.As == obj.AJMP || p.As == obj.ACALL {
				// PC-relative addressing is relative to the end of the instruction,
				// but the relocations applied by the linker are relative to the end
				// of the relocation. Because immediate instruction
				// arguments can follow the PC-relative memory reference in the
				// instruction encoding, the two may not coincide. In this case,
				// adjust addend so that linker can keep relocating relative to the
				// end of the relocation.
				r.Add -= p.Pc + int64(n) - (int64(r.Off) + int64(r.Siz))
			} else if ctxt.Arch.Family == sys.I386 {
				// On 386 PC-relative addressing (for non-call/jmp instructions)
				// assumes that the previous instruction loaded the PC of the end
				// of that instruction into CX, so the adjustment is relative to
				// that.
				r.Add += int64(r.Off) - p.Pc + int64(r.Siz)
			}
		}
		if r.Type == objabi.R_GOTPCREL && ctxt.Arch.Family == sys.I386 {
			// On 386, R_GOTPCREL makes the same assumptions as R_PCREL.
			r.Add += int64(r.Off) - p.Pc + int64(r.Siz)
		}

	}

	if ctxt.Arch.Family == sys.AMD64 && ctxt.Headtype == objabi.Hnacl && p.As != ACMPL && p.As != ACMPQ && p.To.Type == obj.TYPE_REG {
		switch p.To.Reg {
		case REG_SP:
			ab.Put(pstate.naclspfix)
		case REG_BP:
			ab.Put(pstate.naclbpfix)
		}
	}
}

// unpackOps4 extracts 4 operands from p.
func unpackOps4(p *obj.Prog) (arg0, arg1, arg2, dst *obj.Addr) {
	return &p.From, &p.RestArgs[0], &p.RestArgs[1], &p.To
}

// unpackOps5 extracts 5 operands from p.
func unpackOps5(p *obj.Prog) (arg0, arg1, arg2, arg3, dst *obj.Addr) {
	return &p.From, &p.RestArgs[0], &p.RestArgs[1], &p.RestArgs[2], &p.To
}
