package bom

import (
	"fmt"
	"slices"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

// Qualquer estado volta a proposta (recertificação, BOM-07); certificada volta
// a candidata pelo ato que BOM-08 exige da certificação vencida.
var transitions = map[State][]State{
	StateProposta:    {StateCandidata, StateRejeitada},
	StateCandidata:   {StateCertificada, StateRejeitada},
	StateCertificada: {StateDepreciada, StateCandidata},
	StateDepreciada:  {StateNaoSuportada},
}

var initialStates = []State{StateProposta, StateCandidata}

func transitionAllowed(from, to State) bool {
	return from == to || to == StateProposta || slices.Contains(transitions[from], to)
}

func (v *validator) checkTransitions() {
	if v.in.Base == nil {
		return
	}
	before := map[string]State{}
	for _, l := range v.in.Base.located() {
		before[l.key()] = l.entry.State
	}
	current := map[string]bool{}
	for _, l := range v.doc.located() {
		current[l.key()] = true
		to := l.entry.State
		if !isState(to) {
			continue
		}
		from, existed := before[l.key()]
		switch {
		case !existed && !slices.Contains(initialStates, to):
			v.add(rule.CodeB003, l.path, l.entry.Identity,
				fmt.Sprintf("entrada nova em %s: o estado inicial é proposta ou candidata", to))
		case existed && !transitionAllowed(from, to):
			v.add(rule.CodeB003, l.path, l.entry.Identity,
				fmt.Sprintf("transição %s → %s fora da máquina de BOM-07", from, to))
		}
	}
	// Só rejeitada sai do BOM: remover qualquer outra é a transição por omissão
	// que BOM-07 proíbe.
	for _, l := range v.in.Base.located() {
		if current[l.key()] || l.entry.State == StateRejeitada {
			continue
		}
		v.add(rule.CodeB003, l.path, l.entry.Identity,
			fmt.Sprintf("entrada %s@%s em %s removida do BOM: só rejeitada sai", l.entry.Identity, l.entry.Version, l.entry.State))
	}
}
