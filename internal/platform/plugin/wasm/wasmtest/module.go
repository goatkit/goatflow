// Package wasmtest builds minimal WASM plugin modules for tests, so runtime
// limits can be exercised without a TinyGo toolchain.
package wasmtest

import "fmt"

// Fixed guest memory layout.
const (
	manifestAt = 1024
	trueAt     = 2048
	falseAt    = 2056
	okAt       = 2064
	heapStart  = 4096
)

// Module returns a WASM plugin whose gk_register reports manifest (JSON)
// and whose gk_call dispatches on the first byte of the function name:
//
//   - "s…" (e.g. "spin") loops forever;
//   - "g…" (e.g. "grow") grows memory by growPages and returns true, or
//     false when the memory limit refuses;
//   - anything else returns "ok".
//
// Memory starts at one page and declares no maximum, so the runtime's
// memory limit applies. gk_malloc is a bump allocator; there is no gk_free.
func Module(manifest string, growPages uint32) []byte {
	if len(manifest) >= trueAt-manifestAt {
		panic(fmt.Sprintf("wasmtest: manifest of %d bytes does not fit", len(manifest)))
	}
	const (
		i32 = 0x7f
		i64 = 0x7e
	)

	types := vec(
		[]byte{0x60, 0x00, 0x01, i64},                     // 0: gk_register () -> i64
		[]byte{0x60, 0x01, i32, 0x01, i32},                // 1: gk_malloc (i32) -> i32
		[]byte{0x60, 0x04, i32, i32, i32, i32, 0x01, i64}, // 2: gk_call (4×i32) -> i64
	)
	funcs := vec([]byte{0x00}, []byte{0x01}, []byte{0x02})
	memory := vec([]byte{0x00, 0x01}) // min 1 page, no max
	globals := vec(cat([]byte{i32, 0x01, 0x41}, sleb(heapStart), []byte{0x0b}))
	exports := vec(
		cat(name("memory"), []byte{0x02, 0x00}),
		cat(name("gk_register"), []byte{0x00, 0x00}),
		cat(name("gk_malloc"), []byte{0x00, 0x01}),
		cat(name("gk_call"), []byte{0x00, 0x02}),
	)

	register := cat([]byte{0x42}, sleb(packed(manifestAt, len(manifest))), []byte{0x0b})
	malloc := []byte{
		0x23, 0x00, // global.get heap (returned)
		0x23, 0x00, // global.get heap
		0x20, 0x00, // local.get size
		0x6a,       // i32.add
		0x24, 0x00, // global.set heap
		0x0b,
	}
	firstByteIs := func(c byte) []byte {
		return cat([]byte{0x20, 0x00, 0x2d, 0x00, 0x00, 0x41}, sleb(int64(c)), []byte{0x46})
	}
	call := cat(
		firstByteIs('s'),
		[]byte{0x04, 0x40, 0x03, 0x40, 0x0c, 0x00, 0x0b, 0x0b}, // if { loop { br 0 } }
		firstByteIs('g'),
		[]byte{0x04, 0x40, 0x41}, sleb(int64(growPages)),
		[]byte{0x40, 0x00, 0x41, 0x7f, 0x46, 0x04, i64, 0x42}, // memory.grow == -1 ? false : true
		sleb(packed(falseAt, len("false"))),
		[]byte{0x05, 0x42}, sleb(packed(trueAt, len("true"))),
		[]byte{0x0b, 0x0f, 0x0b, 0x42}, sleb(packed(okAt, len(`"ok"`))),
		[]byte{0x0b},
	)
	code := vec(body(register), body(malloc), body(call))

	data := vec(
		segment(manifestAt, manifest),
		segment(trueAt, "true"),
		segment(falseAt, "false"),
		segment(okAt, `"ok"`),
	)

	return cat(
		[]byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00},
		section(1, types), section(3, funcs), section(5, memory), section(6, globals),
		section(7, exports), section(10, code), section(11, data),
	)
}

// packed is the host ABI's ptr<<32|len result.
func packed(ptr, n int) int64 {
	return int64(ptr)<<32 | int64(n)
}

func segment(at int, s string) []byte {
	return cat([]byte{0x00, 0x41}, sleb(int64(at)), []byte{0x0b}, uleb(uint64(len(s))), []byte(s))
}

func body(expr []byte) []byte {
	b := cat([]byte{0x00}, expr) // no locals
	return cat(uleb(uint64(len(b))), b)
}

func section(id byte, content []byte) []byte {
	return cat([]byte{id}, uleb(uint64(len(content))), content)
}

func name(s string) []byte {
	return cat(uleb(uint64(len(s))), []byte(s))
}

func vec(items ...[]byte) []byte {
	return cat(append([][]byte{uleb(uint64(len(items)))}, items...)...)
}

func cat(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func uleb(v uint64) []byte {
	var out []byte
	for {
		b := byte(v & 0x7f) // #nosec G115 -- masked to 7 bits
		v >>= 7
		if v == 0 {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

func sleb(v int64) []byte {
	var out []byte
	for {
		b := byte(v & 0x7f) // #nosec G115 -- masked to 7 bits
		v >>= 7
		if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}
