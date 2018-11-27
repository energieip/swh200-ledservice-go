package rethinkdb

import (
	"fmt"

	test "gopkg.in/check.v1"
	"gopkg.in/rethinkdb/rethinkdb-go.v5/internal/integration/tests"
	"testing"
)

// Hook up gocheck into the gotest runner.
func Test(t *testing.T) { test.TestingT(t) }

type MockSuite struct{}

var _ = test.Suite(&MockSuite{})

func (s *MockSuite) TestMockExecSuccess(c *test.C) {
	mock := NewMock()
	mock.On(DB("test").Table("test").Insert(map[string]string{
		"id": "mocked",
	})).Return(nil, nil)

	err := DB("test").Table("test").Insert(map[string]string{
		"id": "mocked",
	}).Exec(mock)
	c.Assert(err, test.IsNil)
	mock.AssertExpectations(c)
}

func (s *MockSuite) TestMockExecFail(c *test.C) {
	mock := NewMock()
	mock.On(DB("test").Table("test").Insert(map[string]string{
		"id": "mocked",
	})).Return(nil, fmt.Errorf("Expected error"))

	err := DB("test").Table("test").Insert(map[string]string{
		"id": "mocked",
	}).Exec(mock)
	c.Assert(err, test.NotNil)
	mock.AssertExpectations(c)
}

func (s *MockSuite) TestMockRunSuccessSingleResult(c *test.C) {
	mock := NewMock()
	mock.On(DB("test").Table("test").Get("mocked")).Return(map[string]interface{}{
		"id": "mocked",
	}, nil)

	res, err := DB("test").Table("test").Get("mocked").Run(mock)
	c.Assert(err, test.IsNil)

	var response interface{}
	err = res.One(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, map[string]interface{}{"id": "mocked"})
	mock.AssertExpectations(c)

	res.Close()
}

func (s *MockSuite) TestMockRunSuccessMultipleResults(c *test.C) {
	mock := NewMock()
	mock.On(DB("test").Table("test")).Return([]interface{}{
		map[string]interface{}{"id": "mocked"},
	}, nil)

	res, err := DB("test").Table("test").Run(mock)
	c.Assert(err, test.IsNil)

	var response []interface{}
	err = res.All(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, []interface{}{map[string]interface{}{"id": "mocked"}})
	mock.AssertExpectations(c)

	res.Close()
}

func (s *MockSuite) TestMockRunSuccessMultipleResults_type(c *test.C) {
	type document struct {
		Id string
	}

	mock := NewMock()
	mock.On(DB("test").Table("test")).Return([]document{
		document{"mocked"},
	}, nil)

	res, err := DB("test").Table("test").Run(mock)
	c.Assert(err, test.IsNil)

	var response []interface{}
	err = res.All(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, []document{document{"mocked"}})
	mock.AssertExpectations(c)

	res.Close()
}

func (s *MockSuite) TestMockRunMissingMock(c *test.C) {
	mock := NewMock()
	mock.On(DB("test").Table("test")).Return([]interface{}{
		map[string]interface{}{"id": "mocked"},
	}, nil).Once()

	c.Assert(func() {
		c.Assert(DB("test").Table("test").Exec(mock), test.IsNil)
		c.Assert(DB("test").Table("test").Exec(mock), test.IsNil)
	}, test.PanicMatches, ""+
		"rethinkdb: mock: This query was unexpected:(?s:.*)")
	mock.AssertExpectations(c)
}

func (s *MockSuite) TestMockRunMissingQuery(c *test.C) {
	mock := NewMock()
	mock.On(DB("test").Table("test")).Return([]interface{}{
		map[string]interface{}{"id": "mocked"},
	}, nil).Twice()

	c.Assert(DB("test").Table("test").Exec(mock), test.IsNil)

	t := &simpleTestingT{}
	mock.AssertExpectations(t)

	c.Assert(t.Failed(), test.Equals, true)
}

func (s *MockSuite) TestMockRunMissingQuerySingle(c *test.C) {
	mock := NewMock()
	mock.On(DB("test").Table("test")).Return([]interface{}{
		map[string]interface{}{"id": "mocked"},
	}, nil).Once()

	t := &simpleTestingT{}
	mock.AssertExpectations(t)

	c.Assert(t.Failed(), test.Equals, true)
}

func (s *MockSuite) TestMockRunMissingQueryMultiple(c *test.C) {
	mock := NewMock()
	mock.On(DB("test").Table("test")).Return([]interface{}{
		map[string]interface{}{"id": "mocked"},
	}, nil).Twice()

	c.Assert(DB("test").Table("test").Exec(mock), test.IsNil)

	t := &simpleTestingT{}
	mock.AssertExpectations(t)

	c.Assert(t.Failed(), test.Equals, true)
}

func (s *MockSuite) TestMockRunMutlipleQueries(c *test.C) {
	mock := NewMock()
	mock.On(DB("test").Table("test").Get("mocked1")).Return(map[string]interface{}{
		"id": "mocked1",
	}, nil).Times(2)
	mock.On(DB("test").Table("test").Get("mocked2")).Return(map[string]interface{}{
		"id": "mocked2",
	}, nil).Times(1)

	var response interface{}

	// Query 1
	res, err := DB("test").Table("test").Get("mocked1").Run(mock)
	c.Assert(err, test.IsNil)

	err = res.One(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, map[string]interface{}{"id": "mocked1"})

	// Query 2
	res, err = DB("test").Table("test").Get("mocked1").Run(mock)
	c.Assert(err, test.IsNil)

	err = res.One(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, map[string]interface{}{"id": "mocked1"})

	// Query 3
	res, err = DB("test").Table("test").Get("mocked2").Run(mock)
	c.Assert(err, test.IsNil)

	err = res.One(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, map[string]interface{}{"id": "mocked2"})

	mock.AssertExpectations(c)
}

func (s *MockSuite) TestMockQueriesWithFuncs(c *test.C) {
	mock := NewMock()
	mock.On(Expr([]int{2}).Map(func(row Term) interface{} {
		return row.Add(1)
	})).Return([]int{3}, nil).Times(2)
	mock.On(Expr([]int{4}).Map(func(row1, row2 Term) interface{} {
		return row1.Add(1)
	})).Return([]int{5}, nil).Times(1)
	mock.On(Expr([]int{9}).Map(func(row1, row2 Term) interface{} {
		return row2.Add(1)
	})).Return([]int{10}, nil).Times(1)

	var response []int

	// Query 1
	res, err := Expr([]int{2}).Map(func(row Term) interface{} {
		return row.Add(1)
	}).Run(mock)
	c.Assert(err, test.IsNil)

	err = res.All(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, []int{3})

	// Query 2
	res, err = Expr([]int{2}).Map(func(row Term) interface{} {
		return row.Add(1)
	}).Run(mock)
	c.Assert(err, test.IsNil)

	err = res.All(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, []int{3})

	// Query 3
	res, err = Expr([]int{4}).Map(func(row1, row2 Term) interface{} {
		return row1.Add(1)
	}).Run(mock)
	c.Assert(err, test.IsNil)

	err = res.All(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, []int{5})

	// Query 5
	res, err = Expr([]int{9}).Map(func(row1, row2 Term) interface{} {
		return row2.Add(1)
	}).Run(mock)
	c.Assert(err, test.IsNil)

	err = res.All(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, []int{10})

	mock.AssertExpectations(c)
}

func (s *MockSuite) TestMockAnything(c *test.C) {
	mock := NewMock()
	mock.On(MockAnything()).Return("okay", nil).Times(1)
	mock.On(Table("test").MockAnything()).Return("okay2", nil).Times(1)
	mock.On(Table("test").Insert(map[string]interface{}{
		"id": MockAnything(),
	})).Return("okay3", nil).Times(1)
	mock.On(Expr([]interface{}{1, 2, MockAnything()})).Return("okay4", nil).Times(1)

	var response string

	// Query 1
	res, err := Expr("test_1").Run(mock)
	c.Assert(err, test.IsNil)

	err = res.One(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, "okay")

	// Query 2
	res, err = Table("test").Get("mocked1").Run(mock)
	c.Assert(err, test.IsNil)

	err = res.One(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, "okay2")

	// Query 3
	res, err = Table("test").Insert(map[string]interface{}{
		"id": "10ECE456-3C4D-4864-A843-879FCB0D133F",
	}).Run(mock)
	c.Assert(err, test.IsNil)

	err = res.One(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, "okay3")

	// Query 3
	res, err = Expr([]interface{}{1, 2, 3}).Run(mock)
	c.Assert(err, test.IsNil)

	err = res.One(&response)

	c.Assert(err, test.IsNil)
	c.Assert(response, tests.JsonEquals, "okay4")

	mock.AssertExpectations(c)
}

type simpleTestingT struct {
	failed bool
}

func (t *simpleTestingT) Logf(format string, args ...interface{}) {
}
func (t *simpleTestingT) Errorf(format string, args ...interface{}) {
	t.failed = true
}
func (t *simpleTestingT) FailNow() {
	t.failed = true
}
func (t *simpleTestingT) Failed() bool {
	return t.failed
}
