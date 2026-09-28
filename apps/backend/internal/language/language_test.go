package language

import (
	"reflect"
	"testing"
)

func TestResumeLanguagesRequiresSectionOrProficiencyEvidence(t *testing.T) {
	got := ResumeLanguages("Senior backend developer\nLanguages: Russian (native), English — fluent\nExperience: Worked with Spanish customers")
	if want := []string{"en", "ru"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ResumeLanguages() = %v, want %v", got, want)
	}
}

func TestResumeLanguagesInfersDominantCVLanguageWhenNoSpokenLanguagesAreListed(t *testing.T) {
	resume := "Languages: Go (primary), JavaScript / Node.js\n" +
		"I am a backend engineer with experience building reliable software systems. " +
		"I work with teams to design services, solve problems, and deliver production applications."
	if got := ResumeLanguages(resume); !reflect.DeepEqual(got, []string{"en"}) {
		t.Fatalf("ResumeLanguages() = %v, want dominant CV language [en]", got)
	}
}

func TestRequiredLanguagesIgnoresOptionalMention(t *testing.T) {
	got := RequiredLanguages("Fluent English is required. Spanish is a plus. Must be able to speak Mandarin.")
	if want := []string{"en", "zh"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RequiredLanguages() = %v, want %v", got, want)
	}
}

func TestContentLanguageDetectsSpanishAndChineseButNotEnglishWithProductNames(t *testing.T) {
	spanish := "Buscamos un desarrollador con experiencia en diseño, desarrollo de productos y trabajo con nuestro equipo. Ofrecemos una posición remota con excelentes beneficios y oportunidades para crecer."
	if got := ContentLanguage(spanish); got != "es" {
		t.Fatalf("Spanish language = %q", got)
	}
	chinese := "我们正在招聘软件工程师，负责开发和维护云平台。候选人需要丰富的工程经验，并与团队合作。"
	if got := ContentLanguage(chinese); got != "zh" {
		t.Fatalf("Chinese language = %q", got)
	}
	english := "We are looking for a Senior Go Engineer to build reliable software systems and work with our distributed team."
	if got := ContentLanguage(english); got != "en" {
		t.Fatalf("English language = %q", got)
	}
}
