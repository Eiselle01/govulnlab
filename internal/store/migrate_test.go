package store

import "testing"

func TestSplitStatements(t *testing.T) {
	src := `-- 这是一行注释，不能被当成语句
CREATE TABLE IF NOT EXISTS a (id INT);
CREATE TABLE IF NOT EXISTS b (
  name VARCHAR(10) NOT NULL DEFAULT 'x;y'   -- 字符串里的分号不能拆
);
-- 最后一条没有分号
INSERT INTO a VALUES (1)`

	stmts := splitStatements(src)
	if len(stmts) != 3 {
		t.Fatalf("期望 3 条语句，实际 %d: %#v", len(stmts), stmts)
	}
	if stmts[0] != "CREATE TABLE IF NOT EXISTS a (id INT)" {
		t.Errorf("第 1 条解析错误: %q", stmts[0])
	}
	if stmts[2] != "INSERT INTO a VALUES (1)" {
		t.Errorf("第 3 条解析错误: %q", stmts[2])
	}
}
