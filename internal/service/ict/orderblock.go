package ict

// orderblock.go — Order Block detection is now delegated to ta.CalcICT
// via engine.go's convertOrderBlocks(). This file retains only helper
// utilities that may be useful for callers that build on top of the
// ict package.
//
// The canonical Order Block detection algorithm lives in:
//   internal/service/ta/ict.go → detectBullishOBs / detectBearishOBs
