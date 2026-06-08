package stats

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/shopspring/decimal"
)

//go:embed pricing_seed.json
var seedJSON []byte

// perMillion 是定价分母（单价为每百万 token）。
var perMillion = decimal.NewFromInt(1_000_000)

// Price 是某模型每百万 token 的分量单价。
type Price struct {
	Input         decimal.Decimal
	Output        decimal.Decimal
	CacheRead     decimal.Decimal
	CacheCreation decimal.Decimal
}

// Cost 是一次/一组用量的成本分解（decimal，绝不浮点）。
type Cost struct {
	Input         decimal.Decimal
	Output        decimal.Decimal
	CacheRead     decimal.Decimal
	CacheCreation decimal.Decimal
	Total         decimal.Decimal
}

// PriceRow 是从 DB（model_pricing）载入定价的原始行（字符串 decimal）。
type PriceRow struct {
	ModelID                 string
	InputPerMillion         string
	OutputPerMillion        string
	CacheReadPerMillion     string
	CacheCreationPerMillion string
}

// SeedEntry 是内置定价种子的一条（供 dao 以 SeedModelPricing UPSERT，不覆盖 admin 改价）。
type SeedEntry struct {
	ModelID                 string
	InputPerMillion         string
	OutputPerMillion        string
	CacheReadPerMillion     string
	CacheCreationPerMillion string
}

// Table 是模型定价的内存只读视图（启动从 DB 载入，admin 改价后重载）。
type Table struct {
	mu     sync.RWMutex
	prices map[string]Price
}

// NewTable 创建空定价表。
func NewTable() *Table {
	return &Table{prices: map[string]Price{}}
}

// Load 用 DB 行重建定价表（解析失败的分量按 0 处理）。
func (t *Table) Load(rows []PriceRow) {
	m := make(map[string]Price, len(rows))
	for _, r := range rows {
		m[r.ModelID] = Price{
			Input:         parseDec(r.InputPerMillion),
			Output:        parseDec(r.OutputPerMillion),
			CacheRead:     parseDec(r.CacheReadPerMillion),
			CacheCreation: parseDec(r.CacheCreationPerMillion),
		}
	}
	t.mu.Lock()
	t.prices = m
	t.mu.Unlock()
}

// Compute 计算成本：Σ(分量 token × 每百万单价 / 1e6)。
// input 须为已归一的「纯新输入」token（见 BillableInput），故无需再分方言。
// missing=true 表示该模型无定价：成本按 0 返回，token 统计不受影响（仪表盘据此标记 pricing_missing）。
func (t *Table) Compute(model string, input, output, cacheRead, cacheCreation int64) (Cost, bool) {
	t.mu.RLock()
	p, ok := t.prices[model]
	t.mu.RUnlock()
	if !ok {
		return Cost{Input: decimal.Zero, Output: decimal.Zero, CacheRead: decimal.Zero, CacheCreation: decimal.Zero, Total: decimal.Zero}, true
	}
	ic := perMillionCost(input, p.Input)
	oc := perMillionCost(output, p.Output)
	cr := perMillionCost(cacheRead, p.CacheRead)
	cc := perMillionCost(cacheCreation, p.CacheCreation)
	return Cost{Input: ic, Output: oc, CacheRead: cr, CacheCreation: cc, Total: ic.Add(oc).Add(cr).Add(cc)}, false
}

// Has 报告某模型是否有定价（供仪表盘列出缺价模型）。
func (t *Table) Has(model string) bool {
	t.mu.RLock()
	_, ok := t.prices[model]
	t.mu.RUnlock()
	return ok
}

// SeedEntries 解析内置 pricing_seed.json。
func SeedEntries() ([]SeedEntry, error) {
	var raw map[string]struct {
		InputPerMillion         string `json:"input_per_million"`
		OutputPerMillion        string `json:"output_per_million"`
		CacheReadPerMillion     string `json:"cache_read_per_million"`
		CacheCreationPerMillion string `json:"cache_creation_per_million"`
	}
	if err := json.Unmarshal(seedJSON, &raw); err != nil {
		return nil, fmt.Errorf("解析内置定价种子失败: %w", err)
	}
	out := make([]SeedEntry, 0, len(raw))
	for model, p := range raw {
		out = append(out, SeedEntry{
			ModelID:                 model,
			InputPerMillion:         orZero(p.InputPerMillion),
			OutputPerMillion:        orZero(p.OutputPerMillion),
			CacheReadPerMillion:     orZero(p.CacheReadPerMillion),
			CacheCreationPerMillion: orZero(p.CacheCreationPerMillion),
		})
	}
	return out, nil
}

func perMillionCost(tokens int64, perM decimal.Decimal) decimal.Decimal {
	if tokens <= 0 || perM.IsZero() {
		return decimal.Zero
	}
	return decimal.NewFromInt(tokens).Mul(perM).Div(perMillion)
}

func parseDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

func orZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}
