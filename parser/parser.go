package parser

import (
	"fmt"
	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/opcode"
	_ "github.com/pingcap/tidb/types/parser_driver"
	driver "github.com/pingcap/tidb/types/parser_driver"
	"github.com/ryogrid/SamehadaDB/execution/expression"
	"github.com/ryogrid/SamehadaDB/types"
	"reflect"
	"strconv"
	"strings"
)

type QueryType int32

const (
	SELECT QueryType = iota
	CREATE_TABLE
	INSERT
	DELETE
	UPDATE
)

type PredicateExpression struct {
	CompareOperationType_ expression.ComparisonType
	LeftVal               *string
	RightVal              *types.Value
}

type SetExpression struct {
	ColName_     *string
	UpdateValue_ *types.Value
}

type ColDefExpression struct {
	ColName_ *string
	ColType_ *types.TypeID
}

type Visitor interface {
	Enter(n ast.Node) (node ast.Node, skipChildren bool)
	Leave(n ast.Node) (node ast.Node, ok bool)
}

type QueryInfo struct {
	QueryType_         *QueryType
	SelectFields_      []*string
	SetExpression_     *SetExpression
	NewTable_          *string   // for CREATE TABLE
	TargetTable_       *string   // for INSERT, UPDATE
	TargetCols_        []*string // for INSERT
	ColDefExpressions_ []*ColDefExpression
	Values_            []*types.Value // for INSERT
	OnExpressions_     []*PredicateExpression
	FromTable_         *string // for SELECT, DELETE
	JoinTable_         *string
	WhereExpressions_  []*PredicateExpression
}

type SelectFieldsVisitor struct {
	QueryInfo_ *QueryInfo
}

func (v *SelectFieldsVisitor) Enter(in ast.Node) (ast.Node, bool) {
	refVal := reflect.ValueOf(in) // ValueOfでreflect.Value型のオブジェクトを取得
	fmt.Println(refVal.Type())
	switch node := in.(type) {
	case *ast.ColumnName:
		colname := node.Name.String()
		v.QueryInfo_.SelectFields_ = append(v.QueryInfo_.SelectFields_, &colname)
		return in, true
	default:
	}
	return in, false
}

func (v *SelectFieldsVisitor) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

type SimpleSQLVisitor struct {
	QueryInfo_ *QueryInfo
	// member of example code
	colNames []string
}

func NewSimpleSQLVisitor() *SimpleSQLVisitor {
	ret := new(SimpleSQLVisitor)
	qinfo := new(QueryInfo)
	qinfo.QueryType_ = new(QueryType)
	qinfo.SelectFields_ = make([]*string, 0)
	qinfo.SetExpression_ = new(SetExpression)
	qinfo.TargetCols_ = make([]*string, 0)
	qinfo.ColDefExpressions_ = make([]*ColDefExpression, 0)
	qinfo.OnExpressions_ = make([]*PredicateExpression, 0)
	qinfo.WhereExpressions_ = make([]*PredicateExpression, 0)
	ret.QueryInfo_ = qinfo

	return ret
}

func ValueExprToValue(expr *driver.ValueExpr) *types.Value {
	switch expr.Datum.Kind() {
	case 1:
		val_str := expr.String()
		istr := strings.Split(val_str, " ")[1]
		ival, _ := strconv.Atoi(istr)
		ret := types.NewInteger(int32(ival))
		return &ret
	case 8:
		val_str := expr.String()
		fstr := strings.Split(val_str, " ")[1]
		fval, _ := strconv.ParseFloat(fstr, 32)
		ret := types.NewFloat(float32(fval))
		return &ret
	default:
		val_str := expr.String()
		target_str := strings.Split(val_str, " ")[1]
		ret := types.NewVarchar(target_str)
		return &ret
	}
}

func (v *SimpleSQLVisitor) Enter(in ast.Node) (ast.Node, bool) {
	//if name, ok := in.(*ast.ColumnName); ok {
	//	v.colNames = append(v.colNames, name.Name.O)
	//}
	refVal := reflect.ValueOf(in) // ValueOfでreflect.Value型のオブジェクトを取得
	fmt.Println(refVal.Type())

	switch node := in.(type) {
	case *ast.SelectStmt:
		*v.QueryInfo_.QueryType_ = SELECT
	case *ast.CreateTableStmt:
		*v.QueryInfo_.QueryType_ = CREATE_TABLE
	case *ast.InsertStmt:
		*v.QueryInfo_.QueryType_ = INSERT
	case *ast.DeleteStmt:
		*v.QueryInfo_.QueryType_ = DELETE
	case *ast.UpdateStmt:
		*v.QueryInfo_.QueryType_ = UPDATE
	case *ast.FieldList:
	case *ast.SelectField:
		sv := &SelectFieldsVisitor{v.QueryInfo_}
		node.Accept(sv)
		return in, true
	case *ast.TableRefsClause:
	case *ast.Assignment:
	case *ast.Join:
	case *ast.OnCondition:
	case *ast.TableSource:
	case *ast.TableNameExpr:
	case *ast.TableName:
		switch *v.QueryInfo_.QueryType_ {
		case SELECT:
			if v.QueryInfo_.FromTable_ == nil {
				tbname := node.Name.String()
				v.QueryInfo_.FromTable_ = &tbname
			} else {
				tbname := node.Name.String()
				v.QueryInfo_.JoinTable_ = &tbname
			}
		case UPDATE:
			tbname := node.Name.String()
			v.QueryInfo_.TargetTable_ = &tbname
		case INSERT:
			tbname := node.Name.String()
			v.QueryInfo_.TargetTable_ = &tbname
		case DELETE:
			tbname := node.Name.String()
			v.QueryInfo_.FromTable_ = &tbname
		case CREATE_TABLE:
			tbname := node.Name.String()
			v.QueryInfo_.NewTable_ = &tbname
		}
	case *ast.ColumnDef:
		if *v.QueryInfo_.QueryType_ == CREATE_TABLE {
			cdef := new(ColDefExpression)
			cname := node.Name.String()
			cdef.ColName_ = &cname
			col_type := node.Tp.Tp
			switch col_type {
			case 1, 3:
				ctype := types.Integer
				cdef.ColType_ = &ctype
			case 4, 8:
				ctype := types.Float
				cdef.ColType_ = &ctype
			default:
				ctype := types.Varchar
				cdef.ColType_ = &ctype
			}
			v.QueryInfo_.ColDefExpressions_ = append(v.QueryInfo_.ColDefExpressions_, cdef)
		}
	case *ast.ColumnNameExpr:
	case *ast.ColumnName:
		if *v.QueryInfo_.QueryType_ == INSERT {
			cname := node.String()
			v.QueryInfo_.TargetCols_ = append(v.QueryInfo_.TargetCols_, &cname)
		}
	case *ast.BinaryOperationExpr:
		switch node.L.(type) {
		case *ast.ColumnNameExpr: // predicate is composed of single item
			pe := new(PredicateExpression)
			switch node.Op {
			case opcode.EQ:
				pe.CompareOperationType_ = expression.Equal
			case opcode.GT:
				pe.CompareOperationType_ = expression.GreaterThan
			case opcode.GE:
				pe.CompareOperationType_ = expression.GreaterThanOrEqual
			case opcode.LT:
				pe.CompareOperationType_ = expression.LessThan
			case opcode.LE:
				pe.CompareOperationType_ = expression.LessThanOrEqual
			case opcode.NE:
				pe.CompareOperationType_ = expression.NotEqual
			}

			left_val := node.L.(*ast.ColumnNameExpr).Name.String()
			pe.LeftVal = &left_val

			right_node := node.R.(*driver.ValueExpr)
			pe.RightVal = ValueExprToValue(right_node)

			v.QueryInfo_.WhereExpressions_ = append(v.QueryInfo_.WhereExpressions_, pe)
			//pe.RightVal
		default:
			// do nothing
		}

	case *driver.ValueExpr:
		v.QueryInfo_.Values_ = append(v.QueryInfo_.Values_, ValueExprToValue(node))
	default:
		panic("unknown node for visitor")
	}
	return in, false
}

func (v *SimpleSQLVisitor) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func extract(rootNode *ast.StmtNode) []string {
	v := NewSimpleSQLVisitor()
	(*rootNode).Accept(v)
	return v.colNames
}

func parse(sql string) (*ast.StmtNode, error) {
	p := parser.New()

	stmtNodes, _, err := p.Parse(sql, "", "")
	if err != nil {
		return nil, err
	}

	return &stmtNodes[0], nil
}

func TestParsing() {
	//astNode, err := parse("SELECT a, b FROM t WHERE a = daylight")
	//if err != nil {
	//	fmt.Printf("parse error: %v\n", err.Error())
	//	return
	//}
	//fmt.Printf("%v\n", *astNode)

	//if len(os.Args) != 2 {
	//	fmt.Println("usage: colx 'SQL statement'")
	//	return
	//}
	//sql := os.Args[1]
	//sql := "SELECT a, b FROM t WHERE a = 'daylight'"
	//sql := "SELECT a, b FROM t WHERE a = 10"
	//sql := "SELECT a, b FROM t WHERE a = TRUE"
	//sql := "SELECT a, b FROM t WHERE a = 10 AND b = 20 AND c != 'daylight';"
	//sql := "UPDATE employees SET title = 'Mr.' WHERE gender = 'M'"
	sql := "INSERT INTO syain(id,name,romaji) VALUES (1,'鈴木','suzuki');"
	//sql := "DELETE FROM users WHERE id = 10;"
	//sql := "SELECT staff.a, staff.b, staff.c, friend.d FROM staff INNER JOIN friend ON staff.c = friend.c WHERE friend.d = 10;"
	//sql := "CREATE TABLE name_age_list(id INT, name VARCHAR(256), age FLOAT);"
	astNode, err := parse(sql)
	if err != nil {
		fmt.Printf("parse error: %v\n", err.Error())
		return
	}
	fmt.Printf("%v\n", extract(astNode))
}
