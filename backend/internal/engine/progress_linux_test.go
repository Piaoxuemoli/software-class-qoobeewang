//go:build linux

package engine

import (
	"fmt"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
)

// manyFiles 返回含 n 个小文件的目录树。
func manyFiles(n int) testutil.Dir {
	d := testutil.Dir{}
	for i := range n {
		d[fmt.Sprintf("f%03d", i)] = testutil.File{Size: int64(i * 10)}
	}
	return testutil.Dir{"sub": d, "big": testutil.File{Size: 3 * ioBufSize}}
}

// byPhase 按阶段拆分进度事件，并检查阶段顺序与 want 一致。
func byPhase(t *testing.T, ps []Progress, want ...Phase) map[Phase][]Progress {
	t.Helper()
	out := map[Phase][]Progress{}
	var order []Phase
	for _, p := range ps {
		if len(order) == 0 || order[len(order)-1] != p.Phase {
			order = append(order, p.Phase)
		}
		out[p.Phase] = append(out[p.Phase], p)
	}
	if fmt.Sprint(order) != fmt.Sprint(want) {
		t.Errorf("阶段顺序 = %v, want %v", order, want)
	}
	return out
}

// checkMonotonic 检查同一阶段内计数单调不减，最后一次等于 files/bytes。
func checkMonotonic(t *testing.T, ps []Progress, files, bytes int64) {
	t.Helper()
	for i := 1; i < len(ps); i++ {
		if ps[i].Files < ps[i-1].Files || ps[i].Bytes < ps[i-1].Bytes {
			t.Errorf("%s 阶段第 %d 次进度回退: %+v → %+v", ps[i].Phase, i, ps[i-1], ps[i])
		}
	}
	if last := ps[len(ps)-1]; last.Files != files || last.Bytes != bytes {
		t.Errorf("%s 阶段最终进度 %d/%d, want %d/%d", last.Phase, last.Files, last.Bytes, files, bytes)
	}
}

func TestProgress(t *testing.T) {
	t.Run("TC-ENG-060 备份 scan→pack、还原 restore：单调、末次为最终值、pack/restore 带总量", func(t *testing.T) {
		old := progressInterval
		progressInterval = 0 // 不节流，检查每一次报告
		defer func() { progressInterval = old }()
		e := newEnv(t, manyFiles(50))
		bres, brec := e.backup(t, BackupOptions{Compression: container.CompressionHuffman})
		phases := byPhase(t, brec.progress, PhaseScan, PhasePack)
		checkMonotonic(t, phases[PhaseScan], bres.Files, bres.Bytes)
		checkMonotonic(t, phases[PhasePack], bres.Files, bres.Bytes)
		for _, p := range phases[PhaseScan] {
			if p.HasTotal {
				t.Fatalf("scan 阶段不应带总量: %+v", p)
			}
		}
		for _, p := range phases[PhasePack] {
			if !p.HasTotal || p.TotalFiles != bres.Files || p.TotalBytes != bres.Bytes {
				t.Fatalf("pack 阶段总量错误: %+v", p)
			}
		}

		rres, rrec := e.restore(t, bres.Path, nil)
		rp := byPhase(t, rrec.progress, PhaseRestore)[PhaseRestore]
		checkMonotonic(t, rp, rres.Files, rres.Bytes)
		if !rp[0].HasTotal || rp[0].TotalFiles != bres.Files || rp[0].TotalBytes != bres.Bytes {
			t.Errorf("restore 总量应取自容器尾部: %+v", rp[0])
		}
	})

	t.Run("TC-ENG-061 节流：间隔内只发首次与阶段结束时的一次", func(t *testing.T) {
		old := progressInterval
		progressInterval = time.Hour
		defer func() { progressInterval = old }()
		e := newEnv(t, manyFiles(50))
		bres, rec := e.backup(t, BackupOptions{})
		phases := byPhase(t, rec.progress, PhaseScan, PhasePack)
		for ph, ps := range phases {
			if len(ps) != 2 {
				t.Errorf("%s 阶段报告 %d 次, want 2（首次 + 结束）", ph, len(ps))
			}
		}
		checkMonotonic(t, phases[PhasePack], bres.Files, bres.Bytes)
	})

	t.Run("TC-ENG-062 默认节流 ≤10 次/秒", func(t *testing.T) {
		e := newEnv(t, manyFiles(200))
		start := time.Now()
		_, rec := e.backup(t, BackupOptions{Compression: container.CompressionHuffman, Password: []byte("p")})
		elapsed := time.Since(start)
		phases := byPhase(t, rec.progress, PhaseScan, PhasePack)
		for ph, ps := range phases {
			limit := int(elapsed/progressInterval) + 2 // 首次 + 每个间隔一次 + 结束
			if len(ps) > limit {
				t.Errorf("%s 阶段 %v 内报告 %d 次，超过 %d", ph, elapsed, len(ps), limit)
			}
		}
	})
}
