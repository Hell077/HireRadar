// Package language provides conservative language extraction shared by resume
// analysis and candidate matching. It only returns languages when there is
// explicit resume evidence or a reasonably clear language signal in a job.
package language

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type definition struct {
	code    string
	aliases []string
}

var definitions = []definition{
	{code: "ar", aliases: []string{"arabic", "العربية", "арабский"}},
	{code: "de", aliases: []string{"german", "deutsch", "немецкий", "немецкий язык"}},
	{code: "en", aliases: []string{"english", "английский", "английский язык", "inglés", "ingles"}},
	{code: "es", aliases: []string{"spanish", "español", "espanol", "испанский", "испанский язык"}},
	{code: "fr", aliases: []string{"french", "français", "francais", "французский", "французский язык"}},
	{code: "hi", aliases: []string{"hindi", "हिन्दी", "хинди"}},
	{code: "it", aliases: []string{"italian", "italiano", "итальянский"}},
	{code: "ja", aliases: []string{"japanese", "日本語", "японский"}},
	{code: "kk", aliases: []string{"kazakh", "қазақша", "қазақ тілі", "казахский"}},
	{code: "ko", aliases: []string{"korean", "한국어", "корейский"}},
	{code: "nl", aliases: []string{"dutch", "nederlands", "нидерландский"}},
	{code: "pl", aliases: []string{"polish", "polski", "польский"}},
	{code: "pt", aliases: []string{"portuguese", "português", "portugues", "португальский"}},
	{code: "ru", aliases: []string{"russian", "русский", "русский язык"}},
	{code: "tr", aliases: []string{"turkish", "türkçe", "turkce", "турецкий"}},
	{code: "uk", aliases: []string{"ukrainian", "українська", "украинский"}},
	{code: "zh", aliases: []string{"chinese", "mandarin", "cantonese", "中文", "普通话", "китайский"}},
}

var wordsRE = regexp.MustCompile(`[\pL\pN]+`)
var spanishWords = stringSet("el la los las que una uno unas unos para por con como también tambien más mas años anos experiencia requisitos buscamos nuestro nuestra empresa equipo ofrecemos remoto vacante candidato candidata habilidades trabajo desarrollo desarrollador desarrolladora ingeniero ingeniera sueldo jornada puesto salario responsabilidades funciones contratación contratacion")
var englishWords = stringSet("the and for with from this that you your are our will have has experience requirements responsibilities candidate team role work working position")
var proficiencyWords = stringSet("native fluent fluency proficient proficiency advanced intermediate conversational basic beginner professional bilingual speak spoken speaking language languages mother tongue cefr c1 c2 b1 b2 a1 a2")
var requirementWords = stringSet("required requirement mandatory must fluent fluency native proficient proficiency advanced bilingual speak spoken speaking language languages c1 c2 b2")
var optionalWords = stringSet("plus preferred preferably bonus advantage optional desirable")

// ResumeLanguages returns language codes explicitly listed in a language
// section or next to a proficiency marker, plus a clearly detected language
// used throughout the CV. Mentioning a country alone is not enough to infer
// that someone speaks its language.
func ResumeLanguages(text string) []string {
	found := map[string]bool{}
	sectionLines := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if sectionLines > 0 {
				sectionLines--
			}
			continue
		}
		lower := strings.ToLower(line)
		if isLanguageHeading(lower) {
			sectionLines = 6
			if i := strings.IndexAny(line, ":—-|"); i >= 0 {
				addCodes(found, line[i+1:])
			}
			continue
		}
		if sectionLines > 0 && looksLikeHeading(lower) {
			sectionLines = 0
		}
		codes := codesIn(line)
		if sectionLines > 0 || hasAnyWord(lower, proficiencyWords) {
			for _, code := range codes {
				found[code] = true
			}
		}
		if sectionLines > 0 {
			sectionLines--
		}
	}
	if dominant := ContentLanguage(text); dominant != "" {
		found[dominant] = true
	}
	return sortedKeys(found)
}

// RequiredLanguages extracts only explicit language requirements. Optional
// mentions such as "Spanish is a plus" do not restrict eligibility.
func RequiredLanguages(text string) []string {
	found := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		for _, clause := range strings.FieldsFunc(line, func(r rune) bool { return strings.ContainsRune(".;?!|", r) }) {
			lower := strings.ToLower(clause)
			if hasAnyWord(lower, optionalWords) {
				continue
			}
			codes := codesIn(clause)
			if len(codes) == 0 {
				continue
			}
			clauseWords := wordsRE.FindAllString(lower, -1)
			for _, code := range codes {
				if hasRequirementNearLanguage(clauseWords, code, lower) {
					found[code] = true
				}
			}
		}
	}
	return sortedKeys(found)
}

// ContentLanguage identifies a job's primary written language when the text
// provides enough evidence. Empty means unknown and is not a rejection.
func ContentLanguage(text string) string {
	if len(text) > 80_000 {
		text = text[:80_000]
	}
	var letters, han, kana, hangul, cyrillic int
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
		}
		if unicode.In(r, unicode.Han) {
			han++
		}
		if unicode.In(r, unicode.Hiragana, unicode.Katakana) {
			kana++
		}
		if unicode.In(r, unicode.Hangul) {
			hangul++
		}
		if unicode.In(r, unicode.Cyrillic) {
			cyrillic++
		}
	}
	if letters == 0 {
		return ""
	}
	if float64(hangul)/float64(letters) > 0.15 {
		return "ko"
	}
	if float64(kana)/float64(letters) > 0.08 {
		return "ja"
	}
	if kana == 0 && float64(han)/float64(letters) > 0.18 {
		return "zh"
	}
	if float64(cyrillic)/float64(letters) > 0.30 {
		for _, r := range text {
			if strings.ContainsRune("іїєґІЇЄҐ", r) {
				return "uk"
			}
		}
		return "ru"
	}
	counts := map[string]int{}
	for _, token := range wordsRE.FindAllString(strings.ToLower(text), -1) {
		if spanishWords[token] {
			counts["es"]++
		}
		if englishWords[token] {
			counts["en"]++
		}
	}
	if counts["es"] >= 6 && counts["es"]*10 >= len(wordsRE.FindAllString(text, -1)) && counts["es"] > counts["en"]*3/2 {
		return "es"
	}
	if counts["en"] >= 5 && counts["en"] > counts["es"] {
		return "en"
	}
	return ""
}

func hasRequirementNearLanguage(tokens []string, code, clause string) bool {
	aliases := aliasesFor(code)
	for _, alias := range aliases {
		aliasTokens := wordsRE.FindAllString(strings.ToLower(alias), -1)
		for i := 0; i+len(aliasTokens) <= len(tokens); i++ {
			if !sameTokens(tokens[i:i+len(aliasTokens)], aliasTokens) {
				continue
			}
			start, end := i-8, i+len(aliasTokens)+8
			if start < 0 {
				start = 0
			}
			if end > len(tokens) {
				end = len(tokens)
			}
			for _, token := range tokens[start:end] {
				if requirementWords[token] {
					return true
				}
			}
		}
	}
	return strings.Contains(strings.ToLower(clause), "must speak") || strings.Contains(strings.ToLower(clause), "must be fluent")
}

func isLanguageHeading(line string) bool {
	line = strings.TrimLeft(line, "•-* \t")
	for _, prefix := range []string{"language:", "languages:", "language proficiency", "language skills", "spoken languages", "languages spoken", "язык:", "языки:", "языки и", "знание языков"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return line == "languages" || line == "language" || line == "языки" || line == "язык"
}

func looksLikeHeading(line string) bool {
	line = strings.TrimLeft(line, "•-* \t")
	for _, heading := range []string{"experience", "work history", "education", "skills", "projects", "certifications", "опыт", "образование", "навыки", "проекты"} {
		if line == heading || strings.HasPrefix(line, heading+":") {
			return true
		}
	}
	return false
}

func codesIn(text string) []string {
	lower := strings.ToLower(text)
	found := map[string]bool{}
	for _, definition := range definitions {
		for _, alias := range definition.aliases {
			if hasPhrase(lower, alias) {
				found[definition.code] = true
				break
			}
		}
	}
	return sortedKeys(found)
}

func hasPhrase(text, phrase string) bool {
	start := 0
	for {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		end := i + len(phrase)
		beforeOK := i == 0 || !isWordRune(rune(text[i-1]))
		afterOK := end == len(text) || !isWordRune(rune(text[end]))
		if beforeOK && afterOK {
			return true
		}
		start = end
		if start >= len(text) {
			return false
		}
	}
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

func hasAnyWord(text string, wanted map[string]bool) bool {
	for _, token := range wordsRE.FindAllString(strings.ToLower(text), -1) {
		if wanted[token] {
			return true
		}
	}
	return false
}

func addCodes(found map[string]bool, text string) {
	for _, code := range codesIn(text) {
		found[code] = true
	}
}

func aliasesFor(code string) []string {
	for _, definition := range definitions {
		if definition.code == code {
			return definition.aliases
		}
	}
	return nil
}

func sameTokens(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func stringSet(values string) map[string]bool {
	result := make(map[string]bool)
	for _, value := range strings.Fields(values) {
		result[value] = true
	}
	return result
}
