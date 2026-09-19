package sqlm

import "errors"

var (
	// ErrNotFound is returned when a query produced no row at all.
	ErrNotFound error = errors.New("404")

	// ErrNoConnection is returned when no usable connection is available,
	// usually because sqlm.Use()/NewInstance() has not been called (or failed).
	ErrNoConnection = errors.New("sqlm: 没有可用的数据库链接")

	// ErrConnectionLost is returned when the pool can no longer be reached.
	ErrConnectionLost = errors.New("sqlm: 链接已中断")

	// ErrMissingWhere guards UPDATE/DELETE without any condition.
	ErrMissingWhere = errors.New("sqlm: 缺少Where条件")

	// ErrMissingOperation guards Execute() without UPDATE/DELETE intent.
	ErrMissingOperation = errors.New("sqlm: 缺少可执行的操作")

	// ErrMissingColumns is returned when no column has been provided.
	ErrMissingColumns = errors.New("sqlm: 请提供字段")

	// ErrMissingValues is returned when no row has been provided.
	ErrMissingValues = errors.New("sqlm: 请提供数据")

	// ErrColumnsNotMatched is returned when columns and values do not line up.
	ErrColumnsNotMatched = errors.New("sqlm: 请确保column长度统一")

	// ErrNilArgument is returned when a mandatory argument is nil.
	ErrNilArgument = errors.New("sqlm: 参数不能为空")

	// ErrUnsupportedType is returned by Scan when target cannot be handled.
	ErrUnsupportedType = errors.New("sqlm: 不支持的类型")

	// ErrEmptyTableName is returned when the builder has no FROM table.
	ErrEmptyTableName = errors.New("sqlm: 表名不能为空")

	// ErrNotNumeric is returned when a column cannot be read as a number.
	ErrNotNumeric = errors.New("sqlm: 字段不是数字")
)
