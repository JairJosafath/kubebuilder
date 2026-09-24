package emoji

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// selectionQuestions covers the catalog in deterministic, disjoint groups. Each
// Choice stays within both the option limit and the serialized request limit.
func (c *Client) selectionQuestions(ability string) (map[string]question, error) {
	remaining := slices.Sorted(maps.Keys(catalog))
	questions := make(map[string]question)
	for len(remaining) > 0 {
		id := fmt.Sprintf("emoji_%02d", len(questions))
		n := min(maxChoiceOptions, len(remaining))
		if len(remaining)-n == 1 {
			n-- // Leave two options for the final question.
		}
		options := make(map[string]string, n)
		for _, symbol := range remaining[:n] {
			options[symbol] = catalog[symbol]
		}
		q := emojiQuestion(options)
		for {
			if n < 2 {
				return nil, errors.New("jev choice requires at least two options")
			}
			if len(remaining)-n == 1 {
				n--
				delete(options, remaining[n])
				continue
			}
			body, err := c.decisionBody(ability, map[string]question{id: q})
			if err != nil {
				return nil, err
			}
			if len(body) <= maxRequestBytes {
				break
			}
			if n <= 2 {
				return nil, errors.New("jev request exceeds the size limit")
			}
			n--
			delete(options, remaining[n])
		}
		questions[id] = q
		remaining = remaining[n:]
	}
	// Every question advances at most three choices into one final comparison.
	if len(questions)*3 > maxChoiceOptions {
		return nil, errors.New("jev catalog requires too many finalists")
	}
	return questions, nil
}

// packQuestions shares state across independent questions without changing their
// individual option sets or comparing probabilities from different questions.
func (c *Client) packQuestions(ability string, questions map[string]question) ([]map[string]question, error) {
	var requests []map[string]question
	current := make(map[string]question)
	for _, id := range slices.Sorted(maps.Keys(questions)) {
		current[id] = questions[id]
		body, err := c.decisionBody(ability, current)
		if err != nil {
			return nil, err
		}
		if len(current) > maxQuestions || len(body) > maxRequestBytes {
			delete(current, id)
			if len(current) == 0 {
				return nil, errors.New("jev request exceeds the size limit")
			}
			requests = append(requests, current)
			current = map[string]question{id: questions[id]}
			body, err = c.decisionBody(ability, current)
			if err != nil {
				return nil, err
			}
			if len(body) > maxRequestBytes {
				return nil, errors.New("jev request exceeds the size limit")
			}
		}
	}
	if len(current) > 0 {
		requests = append(requests, current)
	}
	return requests, nil
}

// The request hash determines the question order and option counts, so complete
// multi-question decisions can use the existing bounded cache of match slices.
func flattenMatches(answers map[string][]Match) []Match {
	var matches []Match
	for _, id := range slices.Sorted(maps.Keys(answers)) {
		matches = append(matches, answers[id]...)
	}
	return matches
}

func splitCachedMatches(questions map[string]question, matches []Match) map[string][]Match {
	answers := make(map[string][]Match, len(questions))
	for _, id := range slices.Sorted(maps.Keys(questions)) {
		n := len(questions[id].Criteria)
		answers[id] = matches[:n]
		matches = matches[n:]
	}
	return answers
}
