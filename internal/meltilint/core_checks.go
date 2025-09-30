package meltilint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

type methodInfo struct {
	signature string
	pos       token.Pos
}

type interfaceInfo struct {
	methods  map[string]methodInfo
	embedded []token.Pos
	pos      token.Pos
}

type interfaceExpectation struct {
	stdID   string
	methods map[string]string
}

var coreInterfaceExpectations = map[string]interfaceExpectation{
	"Provider": {
		stdID: "STD-01",
		methods: map[string]string{
			"Name":                     "func() string",
			"Capabilities":             "func() ProviderCapabilities",
			"SupportedProtocolVersion": "func() string",
			"Spot":                     "func(context.Context) SpotAPI",
			"LinearFutures":            "func(context.Context) FuturesAPI",
			"InverseFutures":           "func(context.Context) FuturesAPI",
			"WS":                       "func() WS",
			"Close":                    "func() error",
		},
	},
	"SpotAPI": {
		stdID: "STD-02",
		methods: map[string]string{
			"ServerTime":  "func(context.Context) (time.Time, error)",
			"Instruments": "func(context.Context) ([]Instrument, error)",
			"Ticker":      "func(context.Context, string) (Ticker, error)",
			"Balances":    "func(context.Context) ([]Balance, error)",
			"Trades":      "func(context.Context, string, int64) ([]Trade, error)",
			"PlaceOrder":  "func(context.Context, OrderRequest) (Order, error)",
			"GetOrder":    "func(context.Context, string, string, string) (Order, error)",
			"CancelOrder": "func(context.Context, string, string, string) error",
		},
	},
	"FuturesAPI": {
		stdID: "STD-03",
		methods: map[string]string{
			"Instruments": "func(context.Context) ([]Instrument, error)",
			"Ticker":      "func(context.Context, string) (Ticker, error)",
			"PlaceOrder":  "func(context.Context, OrderRequest) (Order, error)",
			"Positions":   "func(context.Context, ...string) ([]Position, error)",
		},
	},
	"WS": {
		stdID: "STD-04",
		methods: map[string]string{
			"SubscribePublic":  "func(context.Context, ...string) (Subscription, error)",
			"SubscribePrivate": "func(context.Context, ...string) (Subscription, error)",
		},
	},
	"Subscription": {
		stdID: "STD-04",
		methods: map[string]string{
			"C":     "func() <-chan Message",
			"Err":   "func() <-chan error",
			"Close": "func() error",
		},
	},
}

func checkCorePackage(pkg *packages.Package) []Issue {
	var issues []Issue
	issues = append(issues, checkCoreInterfaces(pkg)...)
	issues = append(issues, checkCoreModels(pkg)...)
	issues = append(issues, checkCoreEvents(pkg)...)
	issues = append(issues, checkCoreCapabilitiesBitset(pkg)...)
	issues = append(issues, checkCoreDecimalFields(pkg)...)
	issues = append(issues, checkCoreEnums(pkg)...)
	issues = append(issues, checkNoFloatUsage(pkg, "STD-09")...)
	return issues
}

func checkCoreInterfaces(pkg *packages.Package) []Issue {
	interfaces := collectInterfaces(pkg)
	var issues []Issue
	for name, expectation := range coreInterfaceExpectations {
		info, ok := interfaces[name]
		if !ok {
			issues = append(issues, Issue{
				Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
				Package:  pkg.Name,
				Message:  fmt.Sprintf("%s %s interface is missing", expectation.stdID, name),
			})
			continue
		}
		if len(info.embedded) > 0 {
			for _, pos := range info.embedded {
				issues = append(issues, Issue{
					Position: pkg.Fset.Position(pos),
					Package:  pkg.Name,
					Message:  fmt.Sprintf("%s %s must not embed other interfaces", expectation.stdID, name),
				})
			}
		}
		expectedMethods := expectation.methods
		for method, sig := range expectedMethods {
			actual, ok := info.methods[method]
			if !ok {
				issues = append(issues, Issue{
					Position: pkg.Fset.Position(info.pos),
					Package:  pkg.Name,
					Message:  fmt.Sprintf("%s %s.%s is missing", expectation.stdID, name, method),
				})
				continue
			}
			if actual.signature != sig {
				issues = append(issues, Issue{
					Position: pkg.Fset.Position(actual.pos),
					Package:  pkg.Name,
					Message:  fmt.Sprintf("%s %s.%s signature mismatch: want %s got %s", expectation.stdID, name, method, sig, actual.signature),
				})
			}
		}
		for method, infoMethod := range info.methods {
			if _, ok := expectedMethods[method]; !ok {
				issues = append(issues, Issue{
					Position: pkg.Fset.Position(infoMethod.pos),
					Package:  pkg.Name,
					Message:  fmt.Sprintf("%s %s.%s is not part of the frozen interface", expectation.stdID, name, method),
				})
			}
		}
	}
	return issues
}

func collectInterfaces(pkg *packages.Package) map[string]interfaceInfo {
	result := make(map[string]interfaceInfo)
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			iface, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				return true
			}
			info := interfaceInfo{methods: make(map[string]methodInfo), pos: ts.Pos()}
			for _, field := range iface.Methods.List {
				if len(field.Names) == 0 {
					info.embedded = append(info.embedded, field.Pos())
					continue
				}
				name := field.Names[0].Name
				fn, ok := field.Type.(*ast.FuncType)
				if !ok {
					continue
				}
				sig := normalizeFuncType(pkg.Fset, fn)
				info.methods[name] = methodInfo{signature: sig, pos: field.Pos()}
			}
			result[ts.Name.Name] = info
			return false
		})
	}
	return result
}

func normalizeFuncType(fset *token.FileSet, fn *ast.FuncType) string {
	var params []string
	if fn.Params != nil {
		for _, field := range fn.Params.List {
			ptype := exprString(fset, field.Type)
			if len(field.Names) == 0 {
				params = append(params, ptype)
				continue
			}
			for range field.Names {
				params = append(params, ptype)
			}
		}
	}
	var results []string
	if fn.Results != nil {
		for _, field := range fn.Results.List {
			rtype := exprString(fset, field.Type)
			if len(field.Names) == 0 {
				results = append(results, rtype)
				continue
			}
			for range field.Names {
				results = append(results, rtype)
			}
		}
	}
	var b strings.Builder
	b.WriteString("func(")
	b.WriteString(strings.Join(params, ", "))
	b.WriteString(")")
	switch len(results) {
	case 0:
	case 1:
		b.WriteByte(' ')
		b.WriteString(results[0])
	default:
		b.WriteByte(' ')
		b.WriteByte('(')
		b.WriteString(strings.Join(results, ", "))
		b.WriteByte(')')
	}
	return b.String()
}

func checkCoreModels(pkg *packages.Package) []Issue {
	required := []string{"Instrument", "OrderRequest", "Order", "Position", "Ticker", "OrderBook", "Trade", "Kline"}
	typeSpecs := collectTypeSpecs(pkg)
	var issues []Issue
	for _, name := range required {
		ts, ok := typeSpecs[name]
		if !ok {
			issues = append(issues, Issue{
				Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
				Package:  pkg.Name,
				Message:  fmt.Sprintf("STD-05 type %s must be declared", name),
			})
			continue
		}
		if ts.Doc == nil || strings.TrimSpace(ts.Doc.Text()) == "" {
			issues = append(issues, Issue{
				Position: pkg.Fset.Position(ts.Pos()),
				Package:  pkg.Name,
				Message:  fmt.Sprintf("STD-05 type %s must have a doc comment", name),
			})
		}
	}
	return issues
}

func checkCoreEvents(pkg *packages.Package) []Issue {
	required := []string{"TradeEvent", "TickerEvent", "DepthEvent", "OrderEvent", "BalanceEvent"}
	typeSpecs := collectTypeSpecs(pkg)
	var issues []Issue
	for _, name := range required {
		if _, ok := typeSpecs[name]; !ok {
			issues = append(issues, Issue{
				Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
				Package:  pkg.Name,
				Message:  fmt.Sprintf("STD-06 type %s must be declared", name),
			})
		}
	}
	return issues
}

func collectTypeSpecs(pkg *packages.Package) map[string]*ast.TypeSpec {
	result := make(map[string]*ast.TypeSpec)
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if ok {
				result[ts.Name.Name] = ts
			}
			return true
		})
	}
	return result
}

func checkCoreCapabilitiesBitset(pkg *packages.Package) []Issue {
	var issues []Issue
	obj := pkg.Types.Scope().Lookup("ProviderCapabilities")
	if obj == nil {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
			Package:  pkg.Name,
			Message:  "STD-08 ProviderCapabilities type must be declared",
		})
		return issues
	}
	under := obj.Type().Underlying()
	basic, ok := under.(*types.Basic)
	if !ok || basic.Kind() != types.Uint64 {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(obj.Pos()),
			Package:  pkg.Name,
			Message:  "STD-08 ProviderCapabilities must be a uint64 bitset",
		})
	}
	return issues
}

func checkCoreDecimalFields(pkg *packages.Package) []Issue {
	required := map[string][]string{
		"OrderRequest": {"Quantity", "Price"},
		"Order":        {"FilledQty", "AvgPrice"},
		"Ticker":       {"Bid", "Ask"},
		"Position":     {"Quantity", "EntryPrice", "Leverage", "Unrealized"},
		"Trade":        {"Price", "Quantity"},
		"Balance":      {"Available"},
		"DepthLevel":   {"Price", "Qty"},
		"Kline":        {"Open", "High", "Low", "Close", "Volume"},
	}
	typeSpecs := collectTypeSpecs(pkg)
	var issues []Issue
	for typeName, fields := range required {
		ts, ok := typeSpecs[typeName]
		if !ok {
			continue
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			continue
		}
		fieldMap := make(map[string]*ast.Field)
		for _, field := range st.Fields.List {
			for _, name := range field.Names {
				fieldMap[name.Name] = field
			}
		}
		for _, fieldName := range fields {
			field, ok := fieldMap[fieldName]
			if !ok {
				issues = append(issues, Issue{
					Position: pkg.Fset.Position(ts.Pos()),
					Package:  pkg.Name,
					Message:  fmt.Sprintf("STD-10 %s.%s must exist and be *big.Rat", typeName, fieldName),
				})
				continue
			}
			typ := pkg.TypesInfo.TypeOf(field.Type)
			if !isBigRatPointer(typ) {
				issues = append(issues, Issue{
					Position: pkg.Fset.Position(field.Pos()),
					Package:  pkg.Name,
					Message:  fmt.Sprintf("STD-10 %s.%s must be of type *big.Rat", typeName, fieldName),
				})
			}
		}
	}
	return issues
}

func isBigRatPointer(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path() == "math/big" && obj.Name() == "Rat"
}

func checkCoreEnums(pkg *packages.Package) []Issue {
	required := map[string][]string{
		"Market":      {"MarketSpot", "MarketLinearFutures", "MarketInverseFutures"},
		"OrderSide":   {"SideBuy", "SideSell"},
		"OrderType":   {"TypeMarket", "TypeLimit"},
		"TimeInForce": {"TIFGTC", "TIFFOK", "TIFIC"},
	}
	typeSpecs := collectTypeSpecs(pkg)
	constNames := make(map[string]map[string]bool)
	for typ := range required {
		constNames[typ] = make(map[string]bool)
	}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				var typeName string
				if vs.Type != nil {
					typeName = exprString(pkg.Fset, vs.Type)
				}
				for _, name := range vs.Names {
					if typeName == "" {
						if obj := pkg.TypesInfo.Defs[name]; obj != nil {
							typeName = types.TypeString(obj.Type(), func(p *types.Package) string {
								if p == pkg.Types {
									return pkg.Name
								}
								return p.Name()
							})
						}
					}
					plain := strings.TrimPrefix(typeName, pkg.Name+".")
					if names, ok := constNames[plain]; ok {
						names[name.Name] = true
					}
				}
			}
		}
	}
	var issues []Issue
	for typ, expected := range required {
		if _, ok := typeSpecs[typ]; !ok {
			issues = append(issues, Issue{
				Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
				Package:  pkg.Name,
				Message:  fmt.Sprintf("STD-13 type %s must be declared", typ),
			})
			continue
		}
		found := constNames[typ]
		for _, name := range expected {
			if !found[name] {
				issues = append(issues, Issue{
					Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
					Package:  pkg.Name,
					Message:  fmt.Sprintf("STD-13 %s constant %s missing", typ, name),
				})
			}
		}
	}
	return issues
}
