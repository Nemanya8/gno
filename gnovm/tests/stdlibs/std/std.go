package std

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gnolang/gno/gnovm/stdlibs"

	gno "github.com/gnolang/gno/gnovm/pkg/gnolang"
	"github.com/gnolang/gno/gnovm/stdlibs/std"
	"github.com/gnolang/gno/tm2/pkg/crypto"
)

func AssertOriginCall(m *gno.Machine) {
	if !IsOriginCall(m) {
		m.Panic(typedString("invalid non-origin call"))
	}
}

func typedString(s gno.StringValue) gno.TypedValue {
	tv := gno.TypedValue{T: gno.StringType}
	tv.SetString(s)
	return tv
}

func IsOriginCall(m *gno.Machine) bool {
	tname := m.Frames[0].Func.Name
	switch tname {
	case "main": // test is a _filetest
		return len(m.Frames) == 3
	case "runtest": // test is a _test
		return len(m.Frames) == 7
	}
	// support init() in _filetest
	// XXX do we need to distinguish from 'runtest'/_test?
	// XXX pretty hacky even if not.
	if strings.HasPrefix(string(tname), "init.") {
		return len(m.Frames) == 3
	}
	panic("unable to determine if test is a _test or a _filetest")
}

func TestSkipHeights(m *gno.Machine, count int64) {
	ctx := m.Context.(std.ExecContextChain)
	ctx.SetHeight(ctx.Height() + count)
	m.Context = ctx
}

func ClearStoreCache(m *gno.Machine) {
	if gno.IsDebug() && testing.Verbose() {
		m.Store.Print()
		fmt.Println("========================================")
		fmt.Println("CLEAR CACHE (RUNTIME)")
		fmt.Println("========================================")
	}
	m.Store.ClearCache()
	m.PreprocessAllFilesAndSaveBlockNodes()
	if gno.IsDebug() && testing.Verbose() {
		m.Store.Print()
		fmt.Println("========================================")
		fmt.Println("CLEAR CACHE DONE")
		fmt.Println("========================================")
	}
}

func X_callerAt(m *gno.Machine, n int) string {
	if n <= 0 {
		m.Panic(typedString("GetCallerAt requires positive arg"))
		return ""
	}
	// Add 1 to n to account for the GetCallerAt (gno fn) frame.
	n++
	if n > m.NumFrames()-1 {
		// NOTE: the last frame's LastPackage
		// is set to the original non-frame
		// package, so need this check.
		m.Panic(typedString("frame not found"))
		return ""
	}
	if n == m.NumFrames()-1 {
		// This makes it consistent with GetOrigCaller and TestSetOrigCaller.
		ctx := m.Context.(std.ExecContextChain)
		return string(ctx.OrigCaller())
	}
	return string(m.MustLastCallFrame(n).LastPackage.GetPkgAddr().Bech32())
}

func X_testSetOrigCaller(m *gno.Machine, addr string) {
	ctx := m.Context.(std.ExecContextChain)
	ctx.SetOrigCaller(crypto.Bech32Address(addr))
	m.Context = ctx
}

func X_testSetOrigPkgAddr(m *gno.Machine, addr string) {
	ctx := m.Context.(std.ExecContextChain)
	ctx.SetOrigPkgAddr(crypto.Bech32Address(addr))
	m.Context = ctx
}

func X_testSetPrevRealm(m *gno.Machine, pkgPath string) {
	m.Frames[m.NumFrames()-2].LastPackage = &gno.PackageValue{PkgPath: pkgPath}
}

func X_testSetPrevAddr(m *gno.Machine, addr string) {
	// clear all frames to return mocked origin caller
	for i := m.NumFrames() - 1; i > 0; i-- {
		m.Frames[i].LastPackage = nil
	}

	ctx := m.Context.(stdlibs.ExecContextChain)
	ctx.SetOrigCaller(crypto.Bech32Address(addr))
	m.Context = ctx
}

func X_testSetOrigSend(m *gno.Machine,
	sentDenom []string, sentAmt []int64,
	spentDenom []string, spentAmt []int64,
) {
	ctx := m.Context.(std.ExecContextChain)
	ctx.SetOrigSend(std.CompactCoins(sentDenom, sentAmt))
	spent := std.CompactCoins(spentDenom, spentAmt)
	ctx.SetOrigSendSpent(&spent)
	m.Context = ctx
}

func X_testIssueCoins(m *gno.Machine, addr string, denom []string, amt []int64) {
	ctx := m.Context.(std.ExecContextChain)
	banker := ctx.Banker()
	for i := range denom {
		banker.IssueCoin(crypto.Bech32Address(addr), denom[i], amt[i])
	}
}
