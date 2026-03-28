package scanner

import (
	"strings"
	"testing"
)

func TestSymbolExtractor_ExtractGo(t *testing.T) {
	extractor := NewSymbolExtractor()

	tests := []struct {
		name     string
		content  string
		wantSym  []string // expected symbol names
		wantKind []string // expected kinds (parallel to wantSym)
	}{
		{
			name: "basic functions and types",
			content: `package main

type Config struct {
	Name string
}

type Handler interface {
	Serve() error
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) GetName() string {
	return c.Name
}

const MaxSize = 100

var DefaultConfig = &Config{}
`,
			wantSym:  []string{"NewConfig", "GetName", "Config", "Handler", "MaxSize", "DefaultConfig"},
			wantKind: []string{"func", "func", "type", "type", "const", "var"},
		},
		{
			name: "private symbols should not be extracted",
			content: `package main

func doSomething() {}

type privateStruct struct{}

const privateConst = 1

var privateVar int
`,
			wantSym:  nil,
			wantKind: nil,
		},
		{
			name: "empty content",
			content: `package main

// just comments
`,
			wantSym:  nil,
			wantKind: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbols := extractor.ExtractFromContent(tt.content, "go")

			if len(tt.wantSym) == 0 {
				if len(symbols) > 0 {
					t.Errorf("expected no symbols, got %v", symbols)
				}
				return
			}

			if len(symbols) != len(tt.wantSym) {
				t.Errorf("expected %d symbols, got %d: %v", len(tt.wantSym), len(symbols), symbols)
				return
			}

			for i, sym := range symbols {
				if sym.Name != tt.wantSym[i] {
					t.Errorf("symbol[%d]: expected name %q, got %q", i, tt.wantSym[i], sym.Name)
				}
				if sym.Kind != tt.wantKind[i] {
					t.Errorf("symbol[%d]: expected kind %q, got %q", i, tt.wantKind[i], sym.Kind)
				}
			}
		})
	}
}

func TestSymbolExtractor_ExtractJSOrTS(t *testing.T) {
	extractor := NewSymbolExtractor()

	tests := []struct {
		name     string
		content  string
		lang     string
		wantSym  []string
		wantKind []string
	}{
		{
			name: "JavaScript classes and functions",
			lang: "js",
			content: `
class UserService {
	constructor() {}

	getUser(id) {
		return this.fetch(id);
	}
}

function fetchData(url) {
	return fetch(url);
}

const API_KEY = "secret";

const parseData = (data) => JSON.parse(data);
`,
			wantSym:  []string{"UserService", "fetchData", "parseData", "API_KEY"},
			wantKind: []string{"class", "func", "func", "const"},
		},
		{
			name: "TypeScript interfaces and types",
			lang: "ts",
			content: `
interface UserConfig {
	name: string;
}

type UserID = string | number;

class UserManager {
	private config: UserConfig;
}

export async function loadUser(id: UserID): Promise<User> {
	return fetchUser(id);
}

const MAX_RETRIES = 3;
`,
			wantSym:  []string{"UserManager", "loadUser", "UserConfig", "UserID", "MAX_RETRIES"},
			wantKind: []string{"class", "func", "interface", "type", "const"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbols := extractor.ExtractFromContent(tt.content, tt.lang)

			if len(symbols) != len(tt.wantSym) {
				t.Errorf("expected %d symbols, got %d: %v", len(tt.wantSym), len(symbols), symbols)
				return
			}

			for i, sym := range symbols {
				if sym.Name != tt.wantSym[i] {
					t.Errorf("symbol[%d]: expected name %q, got %q", i, tt.wantSym[i], sym.Name)
				}
				if sym.Kind != tt.wantKind[i] {
					t.Errorf("symbol[%d]: expected kind %q, got %q", i, tt.wantKind[i], sym.Kind)
				}
			}
		})
	}
}

func TestSymbolExtractor_ExtractCpp(t *testing.T) {
	extractor := NewSymbolExtractor()

	tests := []struct {
		name        string
		content     string
		wantSym     []string
		wantKind    []string
		wantSymKind map[string]string // alternative: check by name->kind mapping
	}{
		{
			name: "classes and functions",
			content: `
class SettingsManager {
public:
	SettingsManager();
	~SettingsManager();

	void LoadSettings();
	static SettingsManager* GetInstance();
private:
	std::map<std::string, std::string> settings_;
};

struct Options {
	int timeout;
	bool verbose;
};

#define MAX_BUFFER_SIZE 1024

constexpr int kDefaultTimeout = 30;

void SettingsManager::LoadSettings() {
	// implementation
}
`,
			wantSymKind: map[string]string{
				"SettingsManager":  "class",
				"Options":          "struct",
				"LoadSettings":     "func",
				"GetInstance":      "func",
				"MAX_BUFFER_SIZE":  "macro",
				"kDefaultTimeout":  "const",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbols := extractor.ExtractFromContent(tt.content, "cpp")

			// Check by mapping instead of ordered list
			if tt.wantSymKind != nil {
				if len(symbols) != len(tt.wantSymKind) {
					t.Errorf("expected %d symbols, got %d: %v", len(tt.wantSymKind), len(symbols), symbols)
					return
				}

				for _, sym := range symbols {
					expectedKind, ok := tt.wantSymKind[sym.Name]
					if !ok {
						t.Errorf("unexpected symbol name %q", sym.Name)
						continue
					}
					if sym.Kind != expectedKind {
						t.Errorf("symbol %q: expected kind %q, got %q", sym.Name, expectedKind, sym.Kind)
					}
				}
				return
			}

			// Fallback to ordered check
			if len(symbols) != len(tt.wantSym) {
				t.Errorf("expected %d symbols, got %d: %v", len(tt.wantSym), len(symbols), symbols)
				return
			}

			for i, sym := range symbols {
				if sym.Name != tt.wantSym[i] {
					t.Errorf("symbol[%d]: expected name %q, got %q", i, tt.wantSym[i], sym.Name)
				}
				if sym.Kind != tt.wantKind[i] {
					t.Errorf("symbol[%d]: expected kind %q, got %q", i, tt.wantKind[i], sym.Kind)
				}
			}
		})
	}
}

func TestSymbolExtractor_ExtractPython(t *testing.T) {
	extractor := NewSymbolExtractor()

	tests := []struct {
		name     string
		content  string
		wantSym  []string
		wantKind []string
	}{
		{
			name: "classes and functions",
			content: `
class DataProcessor:
	def __init__(self, config):
		self.config = config

	def process(self, data):
		return self.transform(data)

	async def fetch_data(self, url):
		return await self.client.get(url)

	def _private_method(self):
		pass

def parse_config(path):
	with open(path) as f:
		return json.load(f)

def __dunder_method__(self):
	pass
`,
			wantSym:  []string{"DataProcessor", "process", "fetch_data", "parse_config"},
			wantKind: []string{"class", "func", "func", "func"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbols := extractor.ExtractFromContent(tt.content, "py")

			if len(symbols) != len(tt.wantSym) {
				t.Errorf("expected %d symbols, got %d: %v", len(tt.wantSym), len(symbols), symbols)
				return
			}

			for i, sym := range symbols {
				if sym.Name != tt.wantSym[i] {
					t.Errorf("symbol[%d]: expected name %q, got %q", i, tt.wantSym[i], sym.Name)
				}
				if sym.Kind != tt.wantKind[i] {
					t.Errorf("symbol[%d]: expected kind %q, got %q", i, tt.wantKind[i], sym.Kind)
				}
			}
		})
	}
}

func TestSymbolExtractor_ExtractJava(t *testing.T) {
	extractor := NewSymbolExtractor()

	tests := []struct {
		name     string
		content  string
		wantSym  []string
		wantKind []string
	}{
		{
			name: "classes and interfaces",
			content: `
public class UserService {
	private UserRepository repository;

	public User getUser(Long id) {
		return repository.findById(id);
	}

	private void validateUser(User user) {
		// validation logic
	}

	public static UserService createDefault() {
		return new UserService();
	}
}

interface UserRepository {
	User findById(Long id);
	void save(User user);
}
`,
			wantSym:  []string{"UserService", "UserRepository", "getUser", "validateUser", "createDefault", "findById", "save"},
			wantKind: []string{"class", "interface", "func", "func", "func", "func", "func"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbols := extractor.ExtractFromContent(tt.content, "java")

			if len(symbols) != len(tt.wantSym) {
				t.Errorf("expected %d symbols, got %d: %v", len(tt.wantSym), len(symbols), symbols)
				return
			}

			for i, sym := range symbols {
				if sym.Name != tt.wantSym[i] {
					t.Errorf("symbol[%d]: expected name %q, got %q", i, tt.wantSym[i], sym.Name)
				}
				if sym.Kind != tt.wantKind[i] {
					t.Errorf("symbol[%d]: expected kind %q, got %q", i, tt.wantKind[i], sym.Kind)
				}
			}
		})
	}
}

func TestSymbolExtractor_ExtractRust(t *testing.T) {
	extractor := NewSymbolExtractor()

	tests := []struct {
		name     string
		content  string
		wantSym  []string
		wantKind []string
	}{
		{
			name: "structs, enums, and functions",
			content: `
pub struct Config {
	pub name: String,
	pub timeout: u64,
}

enum Status {
	Active,
	Inactive,
}

pub trait Handler {
	fn handle(&self, request: Request) -> Response;
}

pub fn load_config(path: &str) -> Result<Config, Error> {
	let content = std::fs::read_to_string(path)?;
	toml::from_str(&content)
}

async fn fetch_data(url: &str) -> Result<Data, Error> {
	// async implementation
}

const MAX_RETRIES: usize = 3;
`,
			// Note: trait methods like 'handle' are also extracted as functions
			wantSym:  []string{"Config", "Status", "Handler", "handle", "load_config", "fetch_data", "MAX_RETRIES"},
			wantKind: []string{"struct", "enum", "trait", "func", "func", "func", "const"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbols := extractor.ExtractFromContent(tt.content, "rust")

			if len(symbols) != len(tt.wantSym) {
				t.Errorf("expected %d symbols, got %d: %v", len(tt.wantSym), len(symbols), symbols)
				return
			}

			for i, sym := range symbols {
				if sym.Name != tt.wantSym[i] {
					t.Errorf("symbol[%d]: expected name %q, got %q", i, tt.wantSym[i], sym.Name)
				}
				if sym.Kind != tt.wantKind[i] {
					t.Errorf("symbol[%d]: expected kind %q, got %q", i, tt.wantKind[i], sym.Kind)
				}
			}
		})
	}
}

func TestSymbolExtractor_UnknownLanguage(t *testing.T) {
	extractor := NewSymbolExtractor()

	content := "some random content"
	symbols := extractor.ExtractFromContent(content, "unknown")

	if symbols != nil {
		t.Errorf("expected nil for unknown language, got %v", symbols)
	}
}

func TestSymbolExtractor_MaxSymbols(t *testing.T) {
	extractor := NewSymbolExtractor().WithMaxSymbols(2)

	content := `
func First() {}
func Second() {}
func Third() {}
func Fourth() {}
`
	symbols := extractor.ExtractFromContent(content, "go")

	if len(symbols) != 2 {
		t.Errorf("expected 2 symbols with MaxSymbols=2, got %d", len(symbols))
	}
}

func TestSymbolExtractor_FromReader(t *testing.T) {
	extractor := NewSymbolExtractor()

	content := `package main

func Hello() string {
	return "hello"
}
`
	reader := strings.NewReader(content)
	symbols := extractor.Extract(reader, "go")

	if len(symbols) != 1 {
		t.Errorf("expected 1 symbol, got %d", len(symbols))
		return
	}

	if symbols[0].Name != "Hello" {
		t.Errorf("expected symbol name 'Hello', got %q", symbols[0].Name)
	}
}

func TestSymbolExtractor_ExtractSymbolNames(t *testing.T) {
	extractor := NewSymbolExtractor()

	content := `package main

func Alpha() {}
func Beta() {}
`
	names := extractor.ExtractSymbolNames(content, "go")

	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
		return
	}

	expected := []string{"Alpha", "Beta"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("names[%d]: expected %q, got %q", i, expected[i], name)
		}
	}
}
