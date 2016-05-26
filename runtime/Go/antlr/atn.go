package antlr

import "fmt"

// Temporary - for debugging purposes of the Go port
const (
	PortDebug = false
)

var ATNInvalidAltNumber = 0

type ATN struct {
	DecisionToState      []DecisionState
	grammarType          int
	maxTokenType         int
	states               []ATNState
	ruleToStartState     []*RuleStartState
	ruleToStopState      []*RuleStopState
	modeNameToStartState map[string]*TokensStartState
	modeToStartState     []*TokensStartState
	ruleToTokenType      []int
	lexerActions         []LexerAction
}

func NewATN(grammarType int, maxTokenType int) *ATN {

	atn := new(ATN)

	// Used for runtime deserialization of ATNs from strings///
	// The type of the ATN.
	atn.grammarType = grammarType
	// The maximum value for any symbol recognized by a transition in the ATN.
	atn.maxTokenType = maxTokenType
	atn.states = make([]ATNState, 0)
	// Each subrule/rule is a decision point and we must track them so we
	//  can go back later and build DFA predictors for them.  This includes
	//  all the rules, subrules, optional blocks, ()+, ()* etc...
	atn.DecisionToState = make([]DecisionState, 0)
	// Maps from rule index to starting state number.
	atn.ruleToStartState = make([]*RuleStartState, 0)
	// Maps from rule index to stop state number.
	atn.ruleToStopState = nil
	atn.modeNameToStartState = make(map[string]*TokensStartState)
	// For lexer ATNs, atn.maps the rule index to the resulting token type.
	// For parser ATNs, atn.maps the rule index to the generated bypass token
	// type if the
	// {@link ATNDeserializationOptions//isGenerateRuleBypassTransitions}
	// deserialization option was specified otherwise, atn.is {@code nil}.
	atn.ruleToTokenType = nil
	// For lexer ATNs, atn.is an array of {@link LexerAction} objects which may
	// be referenced by action transitions in the ATN.
	atn.lexerActions = nil
	atn.modeToStartState = make([]*TokensStartState, 0)

	return atn

}

// Compute the set of valid tokens that can occur starting in state {@code s}.
//  If {@code ctx} is nil, the set of tokens will not include what can follow
//  the rule surrounding {@code s}. In other words, the set will be
//  restricted to tokens reachable staying within {@code s}'s rule.
func (a *ATN) NextTokensInContext(s ATNState, ctx RuleContext) *IntervalSet {
	var anal = NewLL1Analyzer(a)
	var res = anal.Look(s, nil, ctx)
	return res
}

// Compute the set of valid tokens that can occur starting in {@code s} and
// staying in same rule. {@link Token//EPSILON} is in set if we reach end of
// rule.
func (a *ATN) NextTokensNoContext(s ATNState) *IntervalSet {
	if s.GetNextTokenWithinRule() != nil {
		if PortDebug {
			fmt.Println("DEBUG A")
		}
		return s.GetNextTokenWithinRule()
	}
	if PortDebug {
		fmt.Println("DEBUG 2")
		fmt.Println(a.NextTokensInContext(s, nil))
	}
	s.SetNextTokenWithinRule(a.NextTokensInContext(s, nil))
	s.GetNextTokenWithinRule().readOnly = true
	return s.GetNextTokenWithinRule()
}

func (a *ATN) NextTokens(s ATNState, ctx RuleContext) *IntervalSet {
	if ctx == nil {
		return a.NextTokensNoContext(s)
	}

	return a.NextTokensInContext(s, ctx)
}

func (a *ATN) addState(state ATNState) {
	if state != nil {
		state.SetATN(a)
		state.SetStateNumber(len(a.states))
	}
	a.states = append(a.states, state)
}

func (a *ATN) removeState(state ATNState) {
	a.states[state.GetStateNumber()] = nil // just free mem, don't shift states in list
}

func (a *ATN) defineDecisionState(s DecisionState) int {
	a.DecisionToState = append(a.DecisionToState, s)
	s.setDecision(len(a.DecisionToState) - 1)
	return s.getDecision()
}

func (a *ATN) getDecisionState(decision int) DecisionState {
	if len(a.DecisionToState) == 0 {
		return nil
	}

	return a.DecisionToState[decision]
}

// Computes the set of input symbols which could follow ATN state number
// {@code stateNumber} in the specified full {@code context}. This method
// considers the complete parser context, but does not evaluate semantic
// predicates (i.e. all predicates encountered during the calculation are
// assumed true). If a path in the ATN exists from the starting state to the
// {@link RuleStopState} of the outermost context without Matching any
// symbols, {@link Token//EOF} is added to the returned set.
//
// <p>If {@code context} is {@code nil}, it is treated as
// {@link ParserRuleContext//EMPTY}.</p>
//
// @param stateNumber the ATN state number
// @param context the full parse context
// @return The set of potentially valid input symbols which could follow the
// specified state in the specified context.
// @panics IllegalArgumentException if the ATN does not contain a state with
// number {@code stateNumber}

func (a *ATN) getExpectedTokens(stateNumber int, ctx RuleContext) *IntervalSet {
	if stateNumber < 0 || stateNumber >= len(a.states) {
		panic("Invalid state number.")
	}
	var s = a.states[stateNumber]
	var following = a.NextTokens(s, nil)
	if !following.contains(TokenEpsilon) {
		return following
	}
	var expected = NewIntervalSet()
	expected.addSet(following)
	expected.removeOne(TokenEpsilon)
	for ctx != nil && ctx.GetInvokingState() >= 0 && following.contains(TokenEpsilon) {
		var invokingState = a.states[ctx.GetInvokingState()]
		var rt = invokingState.GetTransitions()[0]
		following = a.NextTokens(rt.(*RuleTransition).followState, nil)
		expected.addSet(following)
		expected.removeOne(TokenEpsilon)
		ctx = ctx.GetParent().(RuleContext)
	}
	if following.contains(TokenEpsilon) {
		expected.addOne(TokenEOF)
	}
	return expected
}
