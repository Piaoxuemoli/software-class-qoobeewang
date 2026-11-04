package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
)

// VerifyOptions 是一次备份验证的选项。
type VerifyOptions struct {
	// Backup 是备份文件路径（名称已由调用方经 repo.Resolve 解析）。
	Backup string
	// Password 用于加密备份；备份未加密时提供则发 warn 并忽略。engine 不保留其引用。
	Password []byte
}

// VerifyResult 是验证结果，对应契约 verify result。
type VerifyResult struct {
	// Files、Dirs、Bytes 是走查归档得到的统计，口径同 backup 结果。
	Files, Dirs, Bytes int64
	// Compressed、Encryption 来自容器头部。
	Compressed bool
	Encryption container.Encryption
	Duration   time.Duration
}

// Verify 验证一个备份文件现在能否完整还原，不写任何文件。三层依次进行：
//  1. 头部与尾部解析（损坏 → ErrCorrupt）；
//  2. 全文件校验值（加密备份校验 HMAC：口令错误或被篡改 → ErrAuth，未给口令 →
//     ErrPasswordRequired；未加密备份校验 SHA-256：损坏 → ErrCorrupt）；
//  3. 解密（如有）→ 解压（如有）→ 走查整个 pack 归档：每个条目的头记录与数据
//     都被完整读出，走查统计与容器尾部交叉核对，不符 → ErrCorrupt。
func Verify(ctx context.Context, opts VerifyOptions, rep Reporter) (VerifyResult, error) {
	start := time.Now()
	r := newRun(ctx, rep)
	res, err := r.verify(opts)
	if err = r.finish(err); err != nil {
		return VerifyResult{}, err
	}
	res.Duration = time.Since(start)
	return res, nil
}

func (r *run) verify(opts VerifyOptions) (VerifyResult, error) {
	f, err := os.Open(opts.Backup)
	if errors.Is(err, fs.ErrNotExist) {
		return VerifyResult{}, fmt.Errorf("%w: %s", ErrBackupNotFound, opts.Backup)
	}
	if err != nil {
		return VerifyResult{}, fmt.Errorf("打开备份: %w", err)
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil {
		return VerifyResult{}, fmt.Errorf("读取备份状态: %w", err)
	} else if !st.Mode().IsRegular() {
		return VerifyResult{}, fmt.Errorf("%w: %s 不是普通文件", container.ErrCorrupt, opts.Backup)
	}
	payload, info, err := r.openBackup(f, opts.Password)
	if err != nil {
		return VerifyResult{}, err
	}
	sum, err := r.walkArchive(payload, info.Summary)
	if err != nil {
		return VerifyResult{}, err
	}
	// 走查统计与容器尾部不一致说明负载与尾部不匹配，还原必然出错或丢数据。
	if sum.Files != info.Summary.Files || sum.Dirs != info.Summary.Dirs || sum.Bytes != info.Summary.Bytes {
		return VerifyResult{}, fmt.Errorf(
			"%w: 负载统计与尾部不符（走查 files=%d dirs=%d bytes=%d，尾部 files=%d dirs=%d bytes=%d）",
			container.ErrCorrupt, sum.Files, sum.Dirs, sum.Bytes,
			info.Summary.Files, info.Summary.Dirs, info.Summary.Bytes)
	}
	return VerifyResult{
		Files: sum.Files, Dirs: sum.Dirs, Bytes: sum.Bytes,
		Compressed: info.Header.Compression != container.CompressionNone,
		Encryption: info.Header.Encryption,
	}, nil
}

// walkArchive 走查整个 pack 归档并按写入口径（packEntries）统计。
// Next 跳过条目数据时字节同样流经解压层，因此负载中的每个字节都被校验到。
func (r *run) walkArchive(payload io.Reader, total container.Summary) (container.Summary, error) {
	var sum container.Summary
	pr := pack.NewReader(payload)
	prog := r.newProgress(PhaseVerify).withTotal(total.Files, total.Bytes)
	for {
		if err := r.cancelled(); err != nil {
			return sum, err
		}
		fi, err := pr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return sum, fmt.Errorf("读取归档: %w", err)
		}
		prog.cur.Current = fi.Path
		if fi.Type == fsmeta.TypeDir {
			sum.Dirs++
		} else {
			sum.Files++
			if fi.Type == fsmeta.TypeRegular && !fi.IsHardlink() {
				sum.Bytes += fi.Size
				prog.cur.Bytes += fi.Size
			}
		}
		prog.cur.Files = sum.Files
		prog.tick()
	}
	prog.flush()
	return sum, nil
}
