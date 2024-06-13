package types

import "encoding/json"

type TraceCallResponse struct {
	Gas         int             `json:"gas"`
	Failed      bool            `json:"failed"`
	ReturnValue string          `json:"returnValue"`
	StructLogs  []TraceCallLogs `json:"structLogs"`
}

type TraceCallLogs struct {
	Pc      int      `json:"pc"`
	Op      string   `json:"op"`
	Gas     int      `json:"gas"`
	GasCost int      `json:"gasCost"`
	Depth   int      `json:"depth"`
	Error   any      `json:"error"`
	Stack   []any    `json:"stack"`
	Memory  any      `json:"memory"`
	Storage struct{} `json:"storage"`
}

type TraceCallTransactionResponse struct {
	Calls   []Call `json:"calls"`
	From    string `json:"from"`
	Gas     string `json:"gas"`
	GasUsed string `json:"gasUsed"`
	Input   string `json:"input"`
	To      string `json:"to"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	TxHash  string `json:"txHash"`
}

type Call struct {
	From    string `json:"from"`
	Gas     string `json:"gas"`
	GasUsed string `json:"gasUsed"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	To      string `json:"to"`
	Type    string `json:"type"`
}

func (t *TraceCallResponse) AsJSON() []byte {
	json, err := json.Marshal(t)
	if err != nil {
		return []byte{}
	}
	return json
}

func (t *TraceCallTransactionResponse) AsJSON() []byte {
	json, err := json.Marshal(t)
	if err != nil {
		return []byte{}
	}
	return json
}
