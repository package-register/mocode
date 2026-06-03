// Package store 提供文件持久化存储功能。
//
// 本文件包含 JSONLWriter 的单元测试，覆盖以下场景：
// - 基本读写操作
// - 并发读写安全性
// - 共享锁和排他锁行为
// - 错误处理
package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_JSONLWriter_Append 测试 Append 方法的基本功能。
func Test_JSONLWriter_Append(t *testing.T) {
	tests := []struct {
		name    string
		values  []any
		wantLen int
		wantErr bool
	}{
		{
			name:    "追加单个值",
			values:  []any{map[string]string{"id": "1"}},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "追加多个值",
			values:  []any{map[string]string{"id": "1"}, map[string]string{"id": "2"}},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "追加空值",
			values:  []any{},
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.jsonl")
			w := NewJSONLWriter(path)

			for _, v := range tt.values {
				err := w.Append(v)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			}

			if !tt.wantErr {
				results, err := w.ReadAll(func() any { return &map[string]string{} })
				require.NoError(t, err)
				assert.Len(t, results, tt.wantLen)
			}
		})
	}
}

// Test_JSONLWriter_Append_DirectoryCreation 测试 Append 自动创建目录。
func Test_JSONLWriter_Append_DirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "test.jsonl")
	w := NewJSONLWriter(path)

	err := w.Append(map[string]string{"key": "value"})
	require.NoError(t, err)

	// 验证文件存在
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

// Test_JSONLWriter_ReadAll 测试 ReadAll 方法。
func Test_JSONLWriter_ReadAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	w := NewJSONLWriter(path)

	// 写入测试数据
	type entry struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	require.NoError(t, w.Append(entry{ID: "1", Name: "Alice"}))
	require.NoError(t, w.Append(entry{ID: "2", Name: "Bob"}))

	// 读取
	results, err := w.ReadAll(func() any { return &entry{} })
	require.NoError(t, err)
	require.Len(t, results, 2)

	e1 := results[0].(*entry)
	assert.Equal(t, "1", e1.ID)
	assert.Equal(t, "Alice", e1.Name)

	e2 := results[1].(*entry)
	assert.Equal(t, "2", e2.ID)
	assert.Equal(t, "Bob", e2.Name)
}

// Test_JSONLWriter_ReadAll_EmptyFile 测试读取空文件。
func Test_JSONLWriter_ReadAll_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	w := NewJSONLWriter(path)

	results, err := w.ReadAll(func() any { return &map[string]string{} })
	require.NoError(t, err)
	assert.Empty(t, results)
}

// Test_JSONLWriter_ReadAll_NonExistentFile 测试读取不存在的文件。
func Test_JSONLWriter_ReadAll_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.jsonl")
	w := NewJSONLWriter(path)

	results, err := w.ReadAll(func() any { return &map[string]string{} })
	require.NoError(t, err)
	assert.Nil(t, results)
}

// Test_JSONLWriter_ScanLines 测试 ScanLines 方法。
func Test_JSONLWriter_ScanLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	w := NewJSONLWriter(path)

	// 写入测试数据
	require.NoError(t, w.Append(map[string]string{"id": "1"}))
	require.NoError(t, w.Append(map[string]string{"id": "2"}))
	require.NoError(t, w.Append(map[string]string{"id": "3"}))

	// 扫描并计数
	count := 0
	err := w.ScanLines(func(line string) error {
		count++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

// Test_JSONLWriter_ScanLines_EarlyReturn 测试 ScanLines 提前返回。
func Test_JSONLWriter_ScanLines_EarlyReturn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	w := NewJSONLWriter(path)

	require.NoError(t, w.Append(map[string]string{"id": "1"}))
	require.NoError(t, w.Append(map[string]string{"id": "2"}))
	require.NoError(t, w.Append(map[string]string{"id": "3"}))

	// 扫描到第二行时停止
	count := 0
	err := w.ScanLines(func(line string) error {
		count++
		if count >= 2 {
			return assert.AnError
		}
		return nil
	})
	assert.Error(t, err)
	assert.Equal(t, 2, count)
}

// Test_JSONLWriter_WriteAll 测试 WriteAll 方法。
func Test_JSONLWriter_WriteAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	w := NewJSONLWriter(path)

	type entry struct {
		Value string `json:"value"`
	}

	values := []any{
		entry{Value: "a"},
		entry{Value: "b"},
		entry{Value: "c"},
	}

	err := w.WriteAll(values)
	require.NoError(t, err)

	results, err := w.ReadAll(func() any { return &entry{} })
	require.NoError(t, err)
	require.Len(t, results, 3)

	assert.Equal(t, "a", results[0].(*entry).Value)
	assert.Equal(t, "b", results[1].(*entry).Value)
	assert.Equal(t, "c", results[2].(*entry).Value)
}

// Test_JSONLWriter_WriteAll_Overwrite 测试 WriteAll 覆盖现有内容。
func Test_JSONLWriter_WriteAll_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	w := NewJSONLWriter(path)

	type entry struct {
		Value string `json:"value"`
	}

	// 第一次写入
	require.NoError(t, w.WriteAll([]any{entry{Value: "old"}}))

	// 第二次写入（覆盖）
	require.NoError(t, w.WriteAll([]any{entry{Value: "new1"}, entry{Value: "new2"}}))

	results, err := w.ReadAll(func() any { return &entry{} })
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "new1", results[0].(*entry).Value)
	assert.Equal(t, "new2", results[1].(*entry).Value)
}

// Test_JSONLWriter_Exists 测试 Exists 方法。
func Test_JSONLWriter_Exists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	w := NewJSONLWriter(path)

	// 文件不存在
	assert.False(t, w.Exists())

	// 创建文件
	require.NoError(t, w.Append(map[string]string{"key": "value"}))

	// 文件存在
	assert.True(t, w.Exists())
}

// Test_JSONLWriter_Remove 测试 Remove 方法。
func Test_JSONLWriter_Remove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	w := NewJSONLWriter(path)

	// 创建文件
	require.NoError(t, w.Append(map[string]string{"key": "value"}))
	assert.True(t, w.Exists())

	// 删除文件
	err := w.Remove()
	require.NoError(t, err)
	assert.False(t, w.Exists())

	// 删除不存在的文件（不应报错）
	err = w.Remove()
	assert.NoError(t, err)
}

// Test_JSONLWriter_ConcurrentReadWrite 测试并发读写安全性。
func Test_JSONLWriter_ConcurrentReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.jsonl")
	w := NewJSONLWriter(path)

	type entry struct {
		ID int `json:"id"`
	}

	const writers = 5
	const readers = 5
	const writesPerGoroutine = 20

	var wg sync.WaitGroup

	// 启动写入者
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				id := writerID*writesPerGoroutine + j
				err := w.Append(entry{ID: id})
				assert.NoError(t, err)
			}
		}(i)
	}

	// 启动读取者
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, err := w.ReadAll(func() any { return &entry{} })
				assert.NoError(t, err)
			}
		}()
	}

	wg.Wait()

	// 验证所有写入都成功
	results, err := w.ReadAll(func() any { return &entry{} })
	require.NoError(t, err)
	assert.Len(t, results, writers*writesPerGoroutine)
}

// Test_JSONLWriter_ConcurrentAppend 测试并发追加的安全性。
func Test_JSONLWriter_ConcurrentAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent_append.jsonl")
	w := NewJSONLWriter(path)

	type entry struct {
		Seq int `json:"seq"`
	}

	const goroutines = 10
	const writes = 50
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < writes; i++ {
				seq := base*writes + i
				err := w.Append(entry{Seq: seq})
				assert.NoError(t, err)
			}
		}(g)
	}

	wg.Wait()

	results, err := w.ReadAll(func() any { return &entry{} })
	require.NoError(t, err)
	assert.Len(t, results, goroutines*writes)

	// 验证没有重复
	seen := make(map[int]bool)
	for _, r := range results {
		e := r.(*entry)
		assert.False(t, seen[e.Seq], "发现重复的 seq=%d", e.Seq)
		seen[e.Seq] = true
	}
}

// Test_JSONLWriter_LargeData 测试大数据量的读写。
func Test_JSONLWriter_LargeData(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过大数据量测试")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "large.jsonl")
	w := NewJSONLWriter(path)

	type entry struct {
		ID   int    `json:"id"`
		Data string `json:"data"`
	}

	// 创建大负载
	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = 'A' + byte(i%26)
	}

	// 写入
	err := w.Append(entry{ID: 1, Data: string(largeData)})
	require.NoError(t, err)

	// 读取
	results, err := w.ReadAll(func() any { return &entry{} })
	require.NoError(t, err)
	require.Len(t, results, 1)

	e := results[0].(*entry)
	assert.Equal(t, 1, e.ID)
	assert.Len(t, e.Data, 10000)
}

// Test_JSONLWriter_Path 测试 Path 方法。
func Test_JSONLWriter_Path(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	w := NewJSONLWriter(path)

	assert.Equal(t, path, w.Path())
}
