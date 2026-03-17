package queryparser

import "testing"

func TestTokenConstants(t *testing.T) {
	// Test that key constants have expected values
	tests := []struct {
		name     string
		constant int
		expected int
	}{
		{"EOF", EOF, 0},
		{"TokenAND", TokenAND, 8},
		{"TokenOR", TokenOR, 9},
		{"TokenNOT", TokenNOT, 10},
		{"TokenPLUS", TokenPLUS, 11},
		{"TokenMINUS", TokenMINUS, 12},
		{"TokenLPAREN", TokenLPAREN, 14},
		{"TokenRPAREN", TokenRPAREN, 15},
		{"TokenCOLON", TokenCOLON, 16},
		{"TokenSTAR", TokenSTAR, 17},
		{"TokenCARAT", TokenCARAT, 18},
		{"TokenQUOTED", TokenQUOTED, 19},
		{"TokenTERM", TokenTERM, 20},
		{"TokenFUZZY_SLOP", TokenFUZZY_SLOP, 21},
		{"TokenPREFIXTERM", TokenPREFIXTERM, 22},
		{"TokenWILDTERM", TokenWILDTERM, 23},
		{"TokenREGEXPTERM", TokenREGEXPTERM, 24},
		{"TokenNUMBER", TokenNUMBER, 25},
		{"TokenRANGEIN_START", TokenRANGEIN_START, 26},
		{"TokenRANGEIN_END", TokenRANGEIN_END, 27},
		{"TokenRANGEEX_START", TokenRANGEEX_START, 28},
		{"TokenRANGEEX_END", TokenRANGEEX_END, 29},
		{"TokenRANGE_TO", TokenRANGE_TO, 30},
		{"TokenRANGE_QUOTED", TokenRANGE_QUOTED, 31},
		{"TokenRANGE_GOOP", TokenRANGE_GOOP, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestGetTokenName(t *testing.T) {
	tests := []struct {
		tokenType int
		expected  string
	}{
		{EOF, "EOF"},
		{TokenAND, "AND"},
		{TokenOR, "OR"},
		{TokenNOT, "NOT"},
		{TokenPLUS, "PLUS"},
		{TokenMINUS, "MINUS"},
		{TokenTERM, "TERM"},
		{TokenQUOTED, "QUOTED"},
		{999, "<UNKNOWN>"}, // Unknown token
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := GetTokenName(tt.tokenType)
			if result != tt.expected {
				t.Errorf("GetTokenName(%d) = %s, want %s", tt.tokenType, result, tt.expected)
			}
		})
	}
}

func TestGetTokenImage(t *testing.T) {
	tests := []struct {
		tokenType int
		expected  string
	}{
		{EOF, "<EOF>"},
		{TokenAND, "\"AND\""},
		{TokenOR, "\"OR\""},
		{TokenNOT, "\"NOT\""},
		{TokenPLUS, "\"+\""},
		{TokenMINUS, "\"-\""},
		{TokenLPAREN, "\"(\""},
		{TokenRPAREN, "\")\""},
		{TokenCOLON, "\":\""},
		{TokenSTAR, "\"*\""},
		{TokenCARAT, "\"^\""},
		{999, "<UNKNOWN>"}, // Unknown token
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := GetTokenImage(tt.tokenType)
			if result != tt.expected {
				t.Errorf("GetTokenImage(%d) = %s, want %s", tt.tokenType, result, tt.expected)
			}
		})
	}
}

func TestIsBooleanOperator(t *testing.T) {
	if !IsBooleanOperator(TokenAND) {
		t.Error("IsBooleanOperator(TokenAND) should be true")
	}
	if !IsBooleanOperator(TokenOR) {
		t.Error("IsBooleanOperator(TokenOR) should be true")
	}
	if !IsBooleanOperator(TokenNOT) {
		t.Error("IsBooleanOperator(TokenNOT) should be true")
	}
	if IsBooleanOperator(TokenTERM) {
		t.Error("IsBooleanOperator(TokenTERM) should be false")
	}
	if IsBooleanOperator(TokenPLUS) {
		t.Error("IsBooleanOperator(TokenPLUS) should be false")
	}
}

func TestIsModifier(t *testing.T) {
	if !IsModifier(TokenPLUS) {
		t.Error("IsModifier(TokenPLUS) should be true")
	}
	if !IsModifier(TokenMINUS) {
		t.Error("IsModifier(TokenMINUS) should be true")
	}
	if !IsModifier(TokenBAREOPER) {
		t.Error("IsModifier(TokenBAREOPER) should be true")
	}
	if IsModifier(TokenAND) {
		t.Error("IsModifier(TokenAND) should be false")
	}
	if IsModifier(TokenTERM) {
		t.Error("IsModifier(TokenTERM) should be false")
	}
}

func TestIsRangeToken(t *testing.T) {
	if !IsRangeToken(TokenRANGEIN_START) {
		t.Error("IsRangeToken(TokenRANGEIN_START) should be true")
	}
	if !IsRangeToken(TokenRANGEIN_END) {
		t.Error("IsRangeToken(TokenRANGEIN_END) should be true")
	}
	if !IsRangeToken(TokenRANGEEX_START) {
		t.Error("IsRangeToken(TokenRANGEEX_START) should be true")
	}
	if !IsRangeToken(TokenRANGEEX_END) {
		t.Error("IsRangeToken(TokenRANGEEX_END) should be true")
	}
	if !IsRangeToken(TokenRANGE_TO) {
		t.Error("IsRangeToken(TokenRANGE_TO) should be true")
	}
	if !IsRangeToken(TokenRANGE_QUOTED) {
		t.Error("IsRangeToken(TokenRANGE_QUOTED) should be true")
	}
	if !IsRangeToken(TokenRANGE_GOOP) {
		t.Error("IsRangeToken(TokenRANGE_GOOP) should be true")
	}
	if IsRangeToken(TokenTERM) {
		t.Error("IsRangeToken(TokenTERM) should be false")
	}
	if IsRangeToken(TokenAND) {
		t.Error("IsRangeToken(TokenAND) should be false")
	}
}

func TestIsTermToken(t *testing.T) {
	if !IsTermToken(TokenTERM) {
		t.Error("IsTermToken(TokenTERM) should be true")
	}
	if !IsTermToken(TokenQUOTED) {
		t.Error("IsTermToken(TokenQUOTED) should be true")
	}
	if !IsTermToken(TokenPREFIXTERM) {
		t.Error("IsTermToken(TokenPREFIXTERM) should be true")
	}
	if !IsTermToken(TokenWILDTERM) {
		t.Error("IsTermToken(TokenWILDTERM) should be true")
	}
	if !IsTermToken(TokenREGEXPTERM) {
		t.Error("IsTermToken(TokenREGEXPTERM) should be true")
	}
	if !IsTermToken(TokenNUMBER) {
		t.Error("IsTermToken(TokenNUMBER) should be true")
	}
	if IsTermToken(TokenAND) {
		t.Error("IsTermToken(TokenAND) should be false")
	}
	if IsTermToken(TokenPLUS) {
		t.Error("IsTermToken(TokenPLUS) should be false")
	}
}

func TestTokenImageLength(t *testing.T) {
	// TokenImage should have enough entries for all defined constants
	if len(TokenImage) <= TokenRANGE_GOOP {
		t.Errorf("TokenImage length %d should be greater than TokenRANGE_GOOP (%d)", len(TokenImage), TokenRANGE_GOOP)
	}
}

func TestTokenNamesCompleteness(t *testing.T) {
	// All token constants should have a name
	constants := []int{
		EOF, NumChar, EscapedChar, TermStartChar, TermChar, Whitespace, QuotedChar,
		TokenAND, TokenOR, TokenNOT, TokenPLUS, TokenMINUS, TokenBAREOPER, TokenLPAREN, TokenRPAREN, TokenCOLON, TokenSTAR, TokenCARAT,
		TokenQUOTED, TokenTERM, TokenFUZZY_SLOP, TokenPREFIXTERM, TokenWILDTERM, TokenREGEXPTERM, TokenNUMBER,
		TokenRANGEIN_START, TokenRANGEIN_END, TokenRANGEEX_START, TokenRANGEEX_END, TokenRANGE_TO, TokenRANGE_QUOTED, TokenRANGE_GOOP,
	}

	for _, c := range constants {
		name := GetTokenName(c)
		if name == "<UNKNOWN>" {
			t.Errorf("Token constant %d missing from TokenNames map", c)
		}
	}
}
