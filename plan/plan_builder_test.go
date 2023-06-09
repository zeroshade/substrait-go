// SPDX-License-Identifier: Apache-2.0

package plan_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	substraitgo "github.com/substrait-io/substrait-go"
	"github.com/substrait-io/substrait-go/expr"
	"github.com/substrait-io/substrait-go/extensions"
	"github.com/substrait-io/substrait-go/plan"
	substraitproto "github.com/substrait-io/substrait-go/proto"
	"github.com/substrait-io/substrait-go/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const versionStruct = `"version": {
	"majorNumber": 0,
	"minorNumber": 29,
	"patchNumber": 0,
	"producer": "substrait-go"
}`

var baseSchema = types.NamedStruct{Names: []string{"a", "b"},
	Struct: types.StructType{
		Nullability: types.NullabilityRequired,
		Types: []types.Type{
			&types.StringType{Nullability: types.NullabilityRequired},
			&types.Float32Type{Nullability: types.NullabilityRequired},
		},
	}}

var baseSchema2 = types.NamedStruct{Names: []string{"x", "y"},
	Struct: types.StructType{
		Nullability: types.NullabilityRequired,
		Types: []types.Type{
			&types.Int32Type{Nullability: types.NullabilityRequired},
			&types.BooleanType{Nullability: types.NullabilityRequired},
		},
	}}

func TestBasicEmitPlan(t *testing.T) {
	b := plan.NewBuilderDefault()
	root, err := b.NamedScanRemap([]string{"test"},
		baseSchema, []int32{1, 0})
	require.NoError(t, err)
	p, err := b.Plan(root, []string{"a", "b"})
	require.NoError(t, err)

	protoPlan, err := p.ToProto()
	require.NoError(t, err)

	roundTrip, err := plan.FromProto(protoPlan, &extensions.DefaultCollection)
	require.NoError(t, err)

	assert.Equal(t, p, roundTrip)
	assert.Equal(t, "NSTRUCT<a: fp32, b: string>", p.GetRoots()[0].RecordType().String())
	assert.Equal(t, roundTrip.GetRoots()[0].RecordType(), p.GetRoots()[0].RecordType())
}

func TestEmitEmptyPlan(t *testing.T) {
	b := plan.NewBuilderDefault()
	root, err := b.NamedScanRemap([]string{"test"},
		baseSchema, []int32{})
	require.NoError(t, err)
	p, err := b.Plan(root, []string{})
	require.NoError(t, err)

	assert.Equal(t, "NSTRUCT<>", p.GetRoots()[0].RecordType().String())

	protoPlan, err := p.ToProto()
	require.NoError(t, err)

	roundTrip, err := plan.FromProto(protoPlan, &extensions.DefaultCollection)
	require.NoError(t, err)

	assert.Equal(t, p, roundTrip)
}

func TestBuildEmitOutOfRangePlan(t *testing.T) {
	b := plan.NewBuilderDefault()
	root, err := b.NamedScanRemap([]string{"test"},
		baseSchema, []int32{2})
	assert.Nil(t, root)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")
}

func checkRoundTrip(t *testing.T, expectedJSON string, p *plan.Plan) {
	protoPlan, err := p.ToProto()
	require.NoError(t, err)

	var expectedProto substraitproto.Plan
	require.NoError(t, protojson.Unmarshal([]byte(expectedJSON), &expectedProto))

	assert.Truef(t, proto.Equal(&expectedProto, protoPlan), "expected: %s\ngot: %s",
		protojson.Format(protoPlan), protojson.Format(&expectedProto))

	roundTrip, err := plan.FromProto(&expectedProto, &extensions.DefaultCollection)
	require.NoError(t, err)

	roundTripProto, err := roundTrip.ToProto()
	require.NoError(t, err)

	assert.Truef(t, proto.Equal(protoPlan, roundTripProto), "expected: %s\ngot: %s",
		protojson.Format(protoPlan), protojson.Format(roundTripProto))
}

func TestAggregateRelPlan(t *testing.T) {
	const expectedJSON = `{
		` + versionStruct + `,
		"extensionUris": [
			{
				"extensionUriAnchor": 1,
				"uri": "https://github.com/substrait-io/substrait/blob/main/extensions/functions_aggregate_generic.yaml"
			}
		],
		"extensions": [
			{
				"extensionFunction": {
					"extensionUriReference": 1,
					"functionAnchor": 1,
					"name": "count"
				}
			}
		],
		"relations": [
			{
				"root": {
					"input": {
						"aggregate": {
							"common": {"direct": {}},
							"input": {
								"read": {
									"common": {"direct": {}},
									"baseSchema": {
										"names": ["a", "b"],
										"struct": {
											"types": [
												{"string": { "nullability": "NULLABILITY_REQUIRED"}},
												{"fp32": { "nullability": "NULLABILITY_REQUIRED"}}
											],
											"nullability": "NULLABILITY_REQUIRED"
										}
									},
									"namedTable": { "names": [ "test" ]}
								}
							},
							"groupings": [
								{
									"groupingExpressions": [
										{ 
											"selection": { 
												"rootReference": {},
												"directReference": { "structField": { "field": 0 }}
											}
										}
									]
								}
							],
							"measures": [
								{
									"measure": {
										"functionReference": 1,
										"outputType": {
											"i64": {
												"nullability": "NULLABILITY_REQUIRED"
											}
										},
										"phase": "AGGREGATION_PHASE_INITIAL_TO_RESULT",
										"invocation": "AGGREGATION_INVOCATION_ALL"
									}
								}
							]							
						}
					},
					"names": ["val", "cnt"]
				}
			}
		]
	}`

	b := plan.NewBuilderDefault()
	aggCount, err := b.AggregateFn(extensions.SubstraitDefaultURIPrefix+"functions_aggregate_generic.yaml",
		"count", nil)
	require.NoError(t, err)
	scan := b.NamedScan([]string{"test"}, baseSchema)
	root, err := b.AggregateColumns(scan, []plan.AggRelMeasure{b.Measure(aggCount, nil)}, 0)
	require.NoError(t, err)

	p, err := b.Plan(root, []string{"val", "cnt"})
	require.NoError(t, err)
	assert.Equal(t, "NSTRUCT<val: string, cnt: i64>", p.GetRoots()[0].RecordType().String())

	checkRoundTrip(t, expectedJSON, p)
}

func TestAggregateNoGrouping(t *testing.T) {
	b := plan.NewBuilderDefault()
	aggCount, err := b.AggregateFn(extensions.SubstraitDefaultURIPrefix+"functions_aggregate_generic.yaml",
		"count", nil)
	require.NoError(t, err)
	scan := b.NamedScan([]string{"test"}, baseSchema)

	root, err := b.AggregateExprs(scan, []plan.AggRelMeasure{b.Measure(aggCount, nil)})
	require.NoError(t, err)

	p, err := b.Plan(root, []string{"cnt"})
	require.NoError(t, err)
	assert.Equal(t, "NSTRUCT<cnt: i64>", p.GetRoots()[0].RecordType().String())
}

func TestAggregateRelErrors(t *testing.T) {
	b := plan.NewBuilderDefault()
	_, err := b.AggregateColumns(nil, nil)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "input Relation must not be nil")

	_, err = b.AggregateExprs(nil, nil)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "input Relation must not be nil")

	scan := b.NamedScan([]string{"test"}, baseSchema)

	_, err = b.AggregateColumns(scan, nil)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "must have at least one grouping expression or measure")

	_, err = b.AggregateExprs(scan, nil)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "must have at least one grouping expression or measure")

	_, err = b.AggregateExprs(scan, nil, nil)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "groupings cannot contain empty expression list or nil expression")

	_, err = b.AggregateExprs(scan, nil, []expr.Expression{nil})
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "groupings cannot contain empty expression list or nil expression")

	_, err = b.AggregateColumns(scan, nil, -1)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidArg)
	assert.ErrorContains(t, err, "cannot create field ref index -1")

	_, err = b.AggregateColumnsRemap(scan, []int32{-1, 5}, nil, 0)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")

	ref, _ := b.RootFieldRef(scan, 0)
	_, err = b.AggregateExprsRemap(scan, []int32{5, -1}, nil, []expr.Expression{ref})
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")
}

func TestCrossRel(t *testing.T) {
	const expectedJSON = `{
		` + versionStruct + `,
		"relations": [
			{
				"root": {
					"input": {
						"cross": {
							"common": {
								"direct": {}
							},
							"left": {
								"read": {
									"common": {"direct": {}},
									"baseSchema": {
										"names": ["a", "b"],
										"struct": {
											"nullability": "NULLABILITY_REQUIRED",
											"types": [
												{ "string": { "nullability": "NULLABILITY_REQUIRED" }},
												{ "fp32": { "nullability": "NULLABILITY_REQUIRED" }}
											]
										}
									},
									"namedTable": {
										"names": [ "test" ]
									}
								}
							},
							"right": {
								"read": {
									"common": {"direct": {}},
									"baseSchema": {
										"names": ["x", "y"],
										"struct": {
											"nullability": "NULLABILITY_REQUIRED",
											"types": [
												{ "i32": { "nullability": "NULLABILITY_REQUIRED" }},
												{ "bool": { "nullability": "NULLABILITY_REQUIRED" }}
											]
										}
									},
									"namedTable": {
										"names": [ "test2" ]
									}
								}
							}
						}
					},
					"names": ["str", "fp", "i", "bool" ]
				}
			}
		]
	}`

	b := plan.NewBuilderDefault()
	left := b.NamedScan([]string{"test"}, baseSchema)
	right := b.NamedScan([]string{"test2"}, baseSchema2)

	root, err := b.Cross(left, right)
	require.NoError(t, err)

	p, err := b.Plan(root, []string{"str", "fp", "i", "bool"})
	require.NoError(t, err)

	assert.Equal(t, "NSTRUCT<str: string, fp: fp32, i: i32, bool: boolean>", p.GetRoots()[0].RecordType().String())

	checkRoundTrip(t, expectedJSON, p)
}

func TestCrossRelErrors(t *testing.T) {
	b := plan.NewBuilderDefault()

	left := b.NamedScan([]string{"test"}, baseSchema)
	right := b.NamedScan([]string{"test2"}, baseSchema2)

	_, err := b.Cross(nil, right)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "input Relation must not be nil")

	_, err = b.Cross(left, nil)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "input Relation must not be nil")

	_, err = b.CrossRemap(left, right, []int32{-1})
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")

	_, err = b.CrossRemap(left, right, []int32{5})
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")
}

func TestFetchRel(t *testing.T) {
	const expectedJSON = `{	
		` + versionStruct + `,
		"relations": [
			{
				"root": {
					"input": {
						"fetch": {
							"common": {"direct": {}},
							"input": {
								"read": {
									"common": {
										"direct": {}
									},
									"baseSchema": {
										"names": ["a"],
										"struct": {
											"nullability": "NULLABILITY_REQUIRED",
											"types": [
												{"string": { "nullability": "NULLABILITY_REQUIRED" }}
											]
										}
									},
									"namedTable": {
										"names": ["test"]
									}
								}
							},
							"offset": 100,
							"count": 50
						}
					},
					"names": ["a"]
				}
			}
		]
	}`

	b := plan.NewBuilderDefault()
	scan := b.NamedScan([]string{"test"}, types.NamedStruct{
		Names: []string{"a"},
		Struct: types.StructType{
			Nullability: types.NullabilityRequired,
			Types: []types.Type{
				&types.StringType{Nullability: substraitproto.Type_NULLABILITY_REQUIRED}},
		},
	})

	fetch, err := b.Fetch(scan, 100, 50)
	require.NoError(t, err)

	p, err := b.Plan(fetch, []string{"a"})
	require.NoError(t, err)

	assert.Equal(t, "NSTRUCT<a: string>", p.GetRoots()[0].RecordType().String())

	checkRoundTrip(t, expectedJSON, p)
}

func TestFetchRelErrors(t *testing.T) {
	b := plan.NewBuilderDefault()

	_, err := b.Fetch(nil, 0, 0)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "input Relation must not be nil")

	scan := b.NamedScan([]string{"test"}, types.NamedStruct{
		Names: []string{"a"},
		Struct: types.StructType{
			Nullability: types.NullabilityRequired,
			Types: []types.Type{
				&types.StringType{Nullability: substraitproto.Type_NULLABILITY_REQUIRED}},
		},
	})

	_, err = b.FetchRemap(scan, 0, 0, []int32{-1})
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")

	_, err = b.FetchRemap(scan, 0, 0, []int32{2})
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")
}

func TestFilterRelation(t *testing.T) {
	const expectedJSON = `{
		` + versionStruct + `,
		"relations": [
			{
				"root": {
					"input": {
						"filter": {
							"common": {
								"direct": {}
							},
							"input": {
								"read": {
									"common": {"direct": {}},
									"baseSchema": {
										"names": ["x", "y"],
										"struct": {
											"types": [
												{"i32": { "nullability": "NULLABILITY_REQUIRED"}},
												{"bool": { "nullability": "NULLABILITY_REQUIRED"}}
											],
											"nullability": "NULLABILITY_REQUIRED"
										}
									},
									"namedTable": { "names": [ "test" ]}
								}
							},
							"condition": {
								"selection": {
									"rootReference": {},
									"directReference": { "structField": { "field": 1 }}
								}
							}
						}
					},
					"names": ["a", "b"]
				}
			}
		]
	}`

	b := plan.NewBuilderDefault()
	scan := b.NamedScan([]string{"test"}, baseSchema2)
	ref, err := b.RootFieldRef(scan, 1)
	require.NoError(t, err)

	filter, err := b.Filter(scan, ref)
	require.NoError(t, err)

	p, err := b.Plan(filter, []string{"a", "b"})
	require.NoError(t, err)

	assert.Equal(t, "NSTRUCT<a: i32, b: boolean>", p.GetRoots()[0].RecordType().String())

	checkRoundTrip(t, expectedJSON, p)
}

func TestFilterRelationErrors(t *testing.T) {
	b := plan.NewBuilderDefault()

	_, err := b.Filter(nil, nil)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "input Relation must not be nil")

	scan := b.NamedScan([]string{"test"}, types.NamedStruct{
		Names: []string{"a"},
		Struct: types.StructType{
			Nullability: types.NullabilityRequired,
			Types: []types.Type{
				&types.StringType{Nullability: substraitproto.Type_NULLABILITY_NULLABLE},
				&types.BooleanType{Nullability: substraitproto.Type_NULLABILITY_NULLABLE}},
		},
	})

	_, err = b.Filter(scan, nil)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "cannot use nil condition in filter relation")

	refStr, _ := b.RootFieldRef(scan, 0)
	refBool, _ := b.RootFieldRef(scan, 1)

	_, err = b.Filter(scan, refStr)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidArg)
	assert.ErrorContains(t, err, "condition for Filter Relation must yield boolean, not string")

	_, err = b.FilterRemap(scan, refBool, []int32{-1})
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")

	_, err = b.FilterRemap(scan, refBool, []int32{3})
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")
}

func TestJoinRelOutputRecordTypes(t *testing.T) {
	const initialJSONFmt = `{
		` + versionStruct + `,
		"relations": [
			{
				"root": {
					"input": {
						"join": {
							"common": {"direct": {}},
							"left": {
								"read": {
									"common": {"direct": {}},
									"baseSchema": {
										"names": ["a", "b"],
										"struct": {
											"nullability": "NULLABILITY_REQUIRED",
											"types": [
												{ "string": { "nullability": "NULLABILITY_REQUIRED" }},
												{ "fp32": { "nullability": "NULLABILITY_REQUIRED" }}
											]
										}
									},
									"namedTable": {
										"names": [ "test" ]
									}
								}
							},
							"right": {
								"read": {
									"common": {"direct": {}},
									"baseSchema": {
										"names": ["x", "y"],
										"struct": {
											"nullability": "NULLABILITY_REQUIRED",
											"types": [
												{ "i32": { "nullability": "NULLABILITY_REQUIRED" }},
												{ "bool": { "nullability": "NULLABILITY_REQUIRED" }}
											]
										}
									},
									"namedTable": {
										"names": [ "test2" ]
									}
								}
							},
							"expression": {
								"selection": {
									"rootReference": {},
									"directReference": { "structField": { "field": 3 }}
								}
							},
							"type": "%s"
						}
					},
					"names": %s
				}
			}
		]
	}`

	tests := []struct {
		joinString   string
		joinType     plan.JoinType
		fields       []string
		recordString string
	}{
		{"JOIN_TYPE_INNER", plan.JoinTypeInner, []string{"a", "b", "c", "d"}, "NSTRUCT<a: string, b: fp32, c: i32, d: boolean>"},
		{"JOIN_TYPE_SEMI", plan.JoinTypeSemi, []string{"a", "b"}, "NSTRUCT<a: string, b: fp32>"},
		{"JOIN_TYPE_OUTER", plan.JoinTypeOuter, []string{"a", "b", "c", "d"}, "NSTRUCT<a: string?, b: fp32?, c: i32?, d: boolean?>"},
		{"JOIN_TYPE_LEFT", plan.JoinTypeLeft, []string{"a", "b", "c", "d"}, "NSTRUCT<a: string, b: fp32, c: i32?, d: boolean?>"},
		{"JOIN_TYPE_RIGHT", plan.JoinTypeRight, []string{"a", "b", "c", "d"}, "NSTRUCT<a: string?, b: fp32?, c: i32, d: boolean>"},
		{"JOIN_TYPE_ANTI", plan.JoinTypeAnti, []string{"a", "b"}, "NSTRUCT<a: string, b: fp32>"},
		{"JOIN_TYPE_SINGLE", plan.JoinTypeSingle, []string{"a", "b", "c", "d"}, "NSTRUCT<a: string, b: fp32, c: i32?, d: boolean?>"},
	}

	for _, tt := range tests {
		t.Run(tt.joinString, func(t *testing.T) {
			b := plan.NewBuilderDefault()
			left := b.NamedScan([]string{"test"}, baseSchema)
			right := b.NamedScan([]string{"test2"}, baseSchema2)

			cond, err := b.JoinedRecordFieldRef(left, right, 3)
			require.NoError(t, err)

			join, err := b.Join(left, right, cond, tt.joinType)
			require.NoError(t, err)

			p, err := b.Plan(join, tt.fields)
			require.NoError(t, err)

			assert.Equal(t, tt.recordString, p.GetRoots()[0].RecordType().String())

			names, _ := json.Marshal(tt.fields)
			checkRoundTrip(t, fmt.Sprintf(initialJSONFmt, tt.joinString, string(names)), p)
		})
	}
}

func TestJoinAndFilterRelation(t *testing.T) {
	const expectedJSON = `{
		` + versionStruct + `,
		"relations": [
			{
				"root": {
					"input": {
						"join": {
							"common": {"direct": {}},
							"left": {
								"read": {
									"common": {"direct": {}},
									"baseSchema": {
										"names": ["a", "b"],
										"struct": {
											"nullability": "NULLABILITY_REQUIRED",
											"types": [
												{ "string": { "nullability": "NULLABILITY_REQUIRED" }},
												{ "fp32": { "nullability": "NULLABILITY_REQUIRED" }}
											]
										}
									},
									"namedTable": {
										"names": [ "test" ]
									}
								}
							},
							"right": {
								"read": {
									"common": {"direct": {}},
									"baseSchema": {
										"names": ["x", "y"],
										"struct": {
											"nullability": "NULLABILITY_REQUIRED",
											"types": [
												{ "i32": { "nullability": "NULLABILITY_REQUIRED" }},
												{ "bool": { "nullability": "NULLABILITY_REQUIRED" }}
											]
										}
									},
									"namedTable": {
										"names": [ "test2" ]
									}
								}
							},
							"expression": {
								"selection": {
									"rootReference": {},
									"directReference": { "structField": { "field": 3 }}
								}
							},
							"postJoinFilter": {
								"selection": {
									"rootReference": {},
									"directReference": { "structField": { "field": 3 }}
								}
							},
							"type": "JOIN_TYPE_INNER"
						}
					},
					"names": ["a", "b", "c", "d"]
				}
			}
		]
	}`

	b := plan.NewBuilderDefault()
	left := b.NamedScan([]string{"test"}, baseSchema)
	right := b.NamedScan([]string{"test2"}, baseSchema2)

	cond, err := b.JoinedRecordFieldRef(left, right, 3)
	require.NoError(t, err)

	join, err := b.JoinAndFilter(left, right, cond, cond, plan.JoinTypeInner)
	require.NoError(t, err)

	p, err := b.Plan(join, []string{"a", "b", "c", "d"})
	require.NoError(t, err)

	checkRoundTrip(t, expectedJSON, p)
}

func TestJoinRelationError(t *testing.T) {
	b := plan.NewBuilderDefault()
	left := b.NamedScan([]string{"test"}, baseSchema)
	right := b.NamedScan([]string{"test2"}, baseSchema2)

	_, err := b.Join(nil, right, nil, plan.JoinTypeUnspecified)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "input Relation must not be nil")

	_, err = b.Join(left, nil, nil, plan.JoinTypeUnspecified)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "input Relation must not be nil")

	_, err = b.Join(left, right, nil, plan.JoinTypeUnspecified)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "cannot use nil condition in filter relation")

	badcond, _ := b.JoinedRecordFieldRef(left, right, 0)
	goodcond, _ := b.JoinedRecordFieldRef(left, right, 3)

	_, err = b.Join(left, right, badcond, plan.JoinTypeUnspecified)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidArg)
	assert.ErrorContains(t, err, "condition for Join Relation must yield boolean, not string")

	_, err = b.Join(left, right, goodcond, plan.JoinTypeUnspecified)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidArg)
	assert.ErrorContains(t, err, "join type must not be unspecified for Join relations")

	_, err = b.JoinRemap(left, right, goodcond, plan.JoinTypeInner, []int32{-1})
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")

	_, err = b.JoinRemap(left, right, goodcond, plan.JoinTypeAnti, []int32{2})
	assert.ErrorIs(t, err, substraitgo.ErrInvalidRel)
	assert.ErrorContains(t, err, "output mapping index out of range")

	_, err = b.JoinAndFilter(left, right, goodcond, badcond, plan.JoinTypeInner)
	assert.ErrorIs(t, err, substraitgo.ErrInvalidArg)
	assert.ErrorContains(t, err, "post join filter must be either nil or yield a boolean, not string")

}
