// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm

import (
	"bytes"
	"github.com/dave/golib/src/cmd/internal/objabi"
	"github.com/dave/golib/src/cmd/link/internal/ld"
	"github.com/dave/golib/src/cmd/link/internal/sym"
	"io"
	"regexp"
)

const (
	I32 = 0x7F
	I64 = 0x7E
	F32 = 0x7D
	F64 = 0x7C
)

const (
	sectionCustom   = 0
	sectionType     = 1
	sectionImport   = 2
	sectionFunction = 3
	sectionTable    = 4
	sectionMemory   = 5
	sectionGlobal   = 6
	sectionExport   = 7
	sectionStart    = 8
	sectionElement  = 9
	sectionCode     = 10
	sectionData     = 11
)

// funcValueOffset is the offset between the PC_F value of a function and the index of the function in WebAssembly
const funcValueOffset = 0x1000 // TODO(neelance): make function addresses play nice with heap addresses

func gentext(ctxt *ld.Link) {
}

type wasmFunc struct {
	Name string
	Type uint32
	Code []byte
}

type wasmFuncType struct {
	Params  []byte
	Results []byte
}

func assignAddress(ctxt *ld.Link, sect *sym.Section, n int, s *sym.Symbol, va uint64, isTramp bool) (*sym.Section, int, uint64) {
	// WebAssembly functions do not live in the same address space as the linear memory.
	// Instead, WebAssembly automatically assigns indices. Imported functions (section "import")
	// have indices 0 to n. They are followed by native functions (sections "function" and "code")
	// with indices n+1 and following.
	//
	// The following rules describe how wasm handles function indices and addresses:
	//   PC_F = funcValueOffset + WebAssembly function index (not including the imports)
	//   s.Value = PC = PC_F<<16 + PC_B
	//
	// The funcValueOffset is necessary to avoid conflicts with expectations
	// that the Go runtime has about function addresses.
	// The field "s.Value" corresponds to the concept of PC at runtime.
	// However, there is no PC register, only PC_F and PC_B. PC_F denotes the function,
	// PC_B the resume point inside of that function. The entry of the function has PC_B = 0.
	s.Sect = sect
	s.Value = int64(funcValueOffset+va/ld.MINFUNC) << 16 // va starts at zero
	va += uint64(ld.MINFUNC)
	return sect, n, va
}

// asmb writes the final WebAssembly module binary.
// Spec: https://webassembly.github.io/spec/core/binary/modules.html
func (pstate *PackageState) asmb(ctxt *ld.Link) {
	if ctxt.Debugvlog != 0 {
		ctxt.Logf("%5.2f asmb\n", pstate.ld.Cputime())
	}

	types := []*wasmFuncType{
		// For normal Go functions the return value is
		// 0 if the function returned normally or
		// 1 if the stack needs to be unwound.
		&wasmFuncType{Results: []byte{I32}},
	}

	// collect host imports (functions that get imported from the WebAssembly host, usually JavaScript)
	hostImports := []*wasmFunc{
		&wasmFunc{
			Name: "debug",
			Type: lookupType(&wasmFuncType{Params: []byte{I32}}, &types),
		},
	}
	hostImportMap := make(map[*sym.Symbol]int64)
	for _, fn := range ctxt.Textp {
		for _, r := range fn.R {
			if r.Type == objabi.R_WASMIMPORT {
				hostImportMap[r.Sym] = int64(len(hostImports))
				hostImports = append(hostImports, &wasmFunc{
					Name: r.Sym.Name,
					Type: lookupType(&wasmFuncType{Params: []byte{I32}}, &types),
				})
			}
		}
	}

	// collect functions with WebAssembly body
	var buildid []byte
	fns := make([]*wasmFunc, len(ctxt.Textp))
	for i, fn := range ctxt.Textp {
		wfn := new(bytes.Buffer)
		if fn.Name == "go.buildid" {
			writeUleb128(wfn, 0) // number of sets of locals
			writeI32Const(wfn, 0)
			wfn.WriteByte(0x0b) // end
			buildid = fn.P
		} else {
			// Relocations have variable length, handle them here.
			off := int32(0)
			for _, r := range fn.R {
				wfn.Write(fn.P[off:r.Off])
				off = r.Off
				switch r.Type {
				case objabi.R_ADDR:
					writeSleb128(wfn, r.Sym.Value+r.Add)
				case objabi.R_CALL:
					writeSleb128(wfn, int64(len(hostImports))+r.Sym.Value>>16-funcValueOffset)
				case objabi.R_WASMIMPORT:
					writeSleb128(wfn, hostImportMap[r.Sym])
				default:
					pstate.ld.Errorf(fn, "bad reloc type %d (%s)", r.Type, pstate.sym.RelocName(ctxt.Arch, r.Type))
					continue
				}
			}
			wfn.Write(fn.P[off:])
		}

		typ := uint32(0)
		if sig, ok := pstate.wasmFuncTypes[fn.Name]; ok {
			typ = lookupType(sig, &types)
		}

		name := pstate.nameRegexp.ReplaceAllString(fn.Name, "_")
		fns[i] = &wasmFunc{Name: name, Type: typ, Code: wfn.Bytes()}
	}

	// look up program entry point
	rt0 := uint32(len(hostImports)) + uint32(ctxt.Syms.ROLookup("_rt0_wasm_js", 0).Value>>16) - funcValueOffset

	ctxt.Out.Write([]byte{0x00, 0x61, 0x73, 0x6d}) // magic
	ctxt.Out.Write([]byte{0x01, 0x00, 0x00, 0x00}) // version

	// Add any buildid early in the binary:
	if len(buildid) != 0 {
		pstate.writeBuildID(ctxt, buildid)
	}

	pstate.writeTypeSec(ctxt, types)
	pstate.writeImportSec(ctxt, hostImports)
	pstate.writeFunctionSec(ctxt, fns)
	pstate.writeTableSec(ctxt, fns)
	pstate.writeMemorySec(ctxt)
	pstate.writeGlobalSec(ctxt)
	pstate.writeExportSec(ctxt, rt0)
	pstate.writeElementSec(ctxt, uint64(len(hostImports)), uint64(len(fns)))
	pstate.writeCodeSec(ctxt, fns)
	pstate.writeDataSec(ctxt)
	if !*pstate.ld.FlagS {
		pstate.writeNameSec(ctxt, len(hostImports), fns)
	}

	ctxt.Out.Flush(pstate.ld)
}

func lookupType(sig *wasmFuncType, types *[]*wasmFuncType) uint32 {
	for i, t := range *types {
		if bytes.Equal(sig.Params, t.Params) && bytes.Equal(sig.Results, t.Results) {
			return uint32(i)
		}
	}
	*types = append(*types, sig)
	return uint32(len(*types) - 1)
}

func writeSecHeader(ctxt *ld.Link, id uint8) int64 {
	ctxt.Out.WriteByte(id)
	sizeOffset := ctxt.Out.Offset()
	ctxt.Out.Write(make([]byte, 5)) // placeholder for length
	return sizeOffset
}

func (pstate *PackageState) writeSecSize(ctxt *ld.Link, sizeOffset int64) {
	endOffset := ctxt.Out.Offset()
	ctxt.Out.SeekSet(pstate.ld, sizeOffset)
	writeUleb128FixedLength(ctxt.Out, uint64(endOffset-sizeOffset-5), 5)
	ctxt.Out.SeekSet(pstate.ld, endOffset)
}

func (pstate *PackageState) writeBuildID(ctxt *ld.Link, buildid []byte) {
	sizeOffset := writeSecHeader(ctxt, sectionCustom)
	writeName(ctxt.Out, "go.buildid")
	ctxt.Out.Write(buildid)
	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeTypeSec writes the section that declares all function types
// so they can be referenced by index.
func (pstate *PackageState) writeTypeSec(ctxt *ld.Link, types []*wasmFuncType) {
	sizeOffset := writeSecHeader(ctxt, sectionType)

	writeUleb128(ctxt.Out, uint64(len(types)))

	for _, t := range types {
		ctxt.Out.WriteByte(0x60) // functype
		writeUleb128(ctxt.Out, uint64(len(t.Params)))
		for _, v := range t.Params {
			ctxt.Out.WriteByte(byte(v))
		}
		writeUleb128(ctxt.Out, uint64(len(t.Results)))
		for _, v := range t.Results {
			ctxt.Out.WriteByte(byte(v))
		}
	}

	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeImportSec writes the section that lists the functions that get
// imported from the WebAssembly host, usually JavaScript.
func (pstate *PackageState) writeImportSec(ctxt *ld.Link, hostImports []*wasmFunc) {
	sizeOffset := writeSecHeader(ctxt, sectionImport)

	writeUleb128(ctxt.Out, uint64(len(hostImports))) // number of imports
	for _, fn := range hostImports {
		writeName(ctxt.Out, "go") // provided by the import object in wasm_exec.js
		writeName(ctxt.Out, fn.Name)
		ctxt.Out.WriteByte(0x00) // func import
		writeUleb128(ctxt.Out, uint64(fn.Type))
	}

	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeFunctionSec writes the section that declares the types of functions.
// The bodies of these functions will later be provided in the "code" section.
func (pstate *PackageState) writeFunctionSec(ctxt *ld.Link, fns []*wasmFunc) {
	sizeOffset := writeSecHeader(ctxt, sectionFunction)

	writeUleb128(ctxt.Out, uint64(len(fns)))
	for _, fn := range fns {
		writeUleb128(ctxt.Out, uint64(fn.Type))
	}

	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeTableSec writes the section that declares tables. Currently there is only a single table
// that is used by the CallIndirect operation to dynamically call any function.
// The contents of the table get initialized by the "element" section.
func (pstate *PackageState) writeTableSec(ctxt *ld.Link, fns []*wasmFunc) {
	sizeOffset := writeSecHeader(ctxt, sectionTable)

	numElements := uint64(funcValueOffset + len(fns))
	writeUleb128(ctxt.Out, 1)           // number of tables
	ctxt.Out.WriteByte(0x70)            // type: anyfunc
	ctxt.Out.WriteByte(0x00)            // no max
	writeUleb128(ctxt.Out, numElements) // min

	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeMemorySec writes the section that declares linear memories. Currently one linear memory is being used.
func (pstate *PackageState) writeMemorySec(ctxt *ld.Link) {
	sizeOffset := writeSecHeader(ctxt, sectionMemory)

	// Linear memory always starts at address zero.
	// The unit of the sizes is "WebAssembly page size", which is 64Ki.
	// The minimum is currently set to 1GB, which is a lot.
	// More memory can be requested with the grow_memory instruction,
	// but this operation currently is rather slow, so we avoid it for now.
	// TODO(neelance): Use lower initial memory size.
	writeUleb128(ctxt.Out, 1)       // number of memories
	ctxt.Out.WriteByte(0x00)        // no maximum memory size
	writeUleb128(ctxt.Out, 1024*16) // minimum (initial) memory size

	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeGlobalSec writes the section that declares global variables.
func (pstate *PackageState) writeGlobalSec(ctxt *ld.Link) {
	sizeOffset := writeSecHeader(ctxt, sectionGlobal)

	globalRegs := []byte{
		I32, // 0: PC_F
		I32, // 1: PC_B
		I32, // 2: SP
		I64, // 3: CTXT
		I64, // 4: g
		I64, // 5: RET0
		I64, // 6: RET1
		I64, // 7: RET2
		I64, // 8: RET3
		I32, // 9: RUN
	}

	writeUleb128(ctxt.Out, uint64(len(globalRegs))) // number of globals

	for _, typ := range globalRegs {
		ctxt.Out.WriteByte(typ)
		ctxt.Out.WriteByte(0x01) // var
		switch typ {
		case I32:
			writeI32Const(ctxt.Out, 0)
		case I64:
			writeI64Const(ctxt.Out, 0)
		}
		ctxt.Out.WriteByte(0x0b) // end
	}

	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeExportSec writes the section that declares exports.
// Exports can be accessed by the WebAssembly host, usually JavaScript.
// Currently _rt0_wasm_js (program entry point) and the linear memory get exported.
func (pstate *PackageState) writeExportSec(ctxt *ld.Link, rt0 uint32) {
	sizeOffset := writeSecHeader(ctxt, sectionExport)

	writeUleb128(ctxt.Out, 2) // number of exports

	writeName(ctxt.Out, "run")          // inst.exports.run in wasm_exec.js
	ctxt.Out.WriteByte(0x00)            // func export
	writeUleb128(ctxt.Out, uint64(rt0)) // funcidx

	writeName(ctxt.Out, "mem") // inst.exports.mem in wasm_exec.js
	ctxt.Out.WriteByte(0x02)   // mem export
	writeUleb128(ctxt.Out, 0)  // memidx

	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeElementSec writes the section that initializes the tables declared by the "table" section.
// The table for CallIndirect gets initialized in a very simple way so that each table index (PC_F value)
// maps linearly to the function index (numImports + PC_F).
func (pstate *PackageState) writeElementSec(ctxt *ld.Link, numImports, numFns uint64) {
	sizeOffset := writeSecHeader(ctxt, sectionElement)

	writeUleb128(ctxt.Out, 1) // number of element segments

	writeUleb128(ctxt.Out, 0) // tableidx
	writeI32Const(ctxt.Out, funcValueOffset)
	ctxt.Out.WriteByte(0x0b) // end

	writeUleb128(ctxt.Out, numFns) // number of entries
	for i := uint64(0); i < numFns; i++ {
		writeUleb128(ctxt.Out, numImports+i)
	}

	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeElementSec writes the section that provides the function bodies for the functions
// declared by the "func" section.
func (pstate *PackageState) writeCodeSec(ctxt *ld.Link, fns []*wasmFunc) {
	sizeOffset := writeSecHeader(ctxt, sectionCode)

	writeUleb128(ctxt.Out, uint64(len(fns))) // number of code entries
	for _, fn := range fns {
		writeUleb128(ctxt.Out, uint64(len(fn.Code)))
		ctxt.Out.Write(fn.Code)
	}

	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeDataSec writes the section that provides data that will be used to initialize the linear memory.
func (pstate *PackageState) writeDataSec(ctxt *ld.Link) {
	sizeOffset := writeSecHeader(ctxt, sectionData)

	sections := []*sym.Section{
		ctxt.Syms.Lookup("runtime.rodata", 0).Sect,
		ctxt.Syms.Lookup("runtime.typelink", 0).Sect,
		ctxt.Syms.Lookup("runtime.itablink", 0).Sect,
		ctxt.Syms.Lookup("runtime.symtab", 0).Sect,
		ctxt.Syms.Lookup("runtime.pclntab", 0).Sect,
		ctxt.Syms.Lookup("runtime.noptrdata", 0).Sect,
		ctxt.Syms.Lookup("runtime.data", 0).Sect,
	}

	writeUleb128(ctxt.Out, uint64(len(sections))) // number of data entries

	for _, sec := range sections {
		writeUleb128(ctxt.Out, 0) // memidx
		writeI32Const(ctxt.Out, int32(sec.Vaddr))
		ctxt.Out.WriteByte(0x0b) // end
		writeUleb128(ctxt.Out, uint64(sec.Length))
		pstate.ld.Datblk(ctxt, int64(sec.Vaddr), int64(sec.Length))
	}

	pstate.writeSecSize(ctxt, sizeOffset)
}

// writeNameSec writes an optional section that assigns names to the functions declared by the "func" section.
// The names are only used by WebAssembly stack traces, debuggers and decompilers.
// TODO(neelance): add symbol table of DATA symbols
func (pstate *PackageState) writeNameSec(ctxt *ld.Link, firstFnIndex int, fns []*wasmFunc) {
	sizeOffset := writeSecHeader(ctxt, sectionCustom)
	writeName(ctxt.Out, "name")

	sizeOffset2 := writeSecHeader(ctxt, 0x01) // function names
	writeUleb128(ctxt.Out, uint64(len(fns)))
	for i, fn := range fns {
		writeUleb128(ctxt.Out, uint64(firstFnIndex+i))
		writeName(ctxt.Out, fn.Name)
	}
	pstate.writeSecSize(ctxt, sizeOffset2)

	pstate.writeSecSize(ctxt, sizeOffset)
}

type nameWriter interface {
	io.ByteWriter
	io.Writer
}

func writeI32Const(w io.ByteWriter, v int32) {
	w.WriteByte(0x41) // i32.const
	writeSleb128(w, int64(v))
}

func writeI64Const(w io.ByteWriter, v int64) {
	w.WriteByte(0x42) // i64.const
	writeSleb128(w, v)
}

func writeName(w nameWriter, name string) {
	writeUleb128(w, uint64(len(name)))
	w.Write([]byte(name))
}

func writeUleb128(w io.ByteWriter, v uint64) {
	more := true
	for more {
		c := uint8(v & 0x7f)
		v >>= 7
		more = v != 0
		if more {
			c |= 0x80
		}
		w.WriteByte(c)
	}
}

func writeUleb128FixedLength(w io.ByteWriter, v uint64, length int) {
	for i := 0; i < length; i++ {
		c := uint8(v & 0x7f)
		v >>= 7
		if i < length-1 {
			c |= 0x80
		}
		w.WriteByte(c)
	}
	if v != 0 {
		panic("writeUleb128FixedLength: length too small")
	}
}

func writeSleb128(w io.ByteWriter, v int64) {
	more := true
	for more {
		c := uint8(v & 0x7f)
		s := uint8(v & 0x40)
		v >>= 7
		more = !((v == 0 && s == 0) || (v == -1 && s != 0))
		if more {
			c |= 0x80
		}
		w.WriteByte(c)
	}
}
