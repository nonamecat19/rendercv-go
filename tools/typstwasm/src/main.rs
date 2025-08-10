//! A WASI shim around the typst 0.14 compiler.
//!
//! Mirrors `third_party/rendercv/src/rendercv/renderer/pdf_png.py`'s
//! `get_typst_compiler` (`:154-186`): a root directory, a list of font folders,
//! and a package path laid out as `preview/<name>/<version>/`.
//!
//! Everything the compiler reads comes through WASI preopens, so the host
//! decides what is visible.

use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::Mutex;

use typst::diag::{FileError, FileResult, SourceResult};
use typst::foundations::{Bytes, Datetime, Smart};
use typst::layout::PagedDocument;
use typst::syntax::{FileId, Source, VirtualPath};
use typst::text::{Font, FontBook};
use typst::utils::LazyHash;
use typst::{Library, LibraryExt, World};

struct ShimWorld {
    library: LazyHash<Library>,
    book: LazyHash<FontBook>,
    fonts: Vec<Font>,
    root: PathBuf,
    package_root: PathBuf,
    main: FileId,
    sources: Mutex<HashMap<FileId, Source>>,
    files: Mutex<HashMap<FileId, Bytes>>,
    today: Option<Datetime>,
}

impl ShimWorld {
    /// Resolve a file id to a host path, honouring the package layout upstream
    /// builds in `get_package_path` (`pdf_png.py:114-146`).
    fn locate(&self, id: FileId) -> FileResult<PathBuf> {
        let vpath = id.vpath();
        match id.package() {
            None => vpath
                .resolve(&self.root)
                .ok_or_else(|| FileError::AccessDenied),
            Some(spec) => {
                let dir = self
                    .package_root
                    .join(spec.namespace.as_str())
                    .join(spec.name.as_str())
                    .join(spec.version.to_string());
                vpath.resolve(&dir).ok_or_else(|| FileError::AccessDenied)
            }
        }
    }

    fn read(&self, id: FileId) -> FileResult<Vec<u8>> {
        let path = self.locate(id)?;
        std::fs::read(&path).map_err(|err| FileError::from_io(err, &path))
    }
}

impl World for ShimWorld {
    fn library(&self) -> &LazyHash<Library> {
        &self.library
    }

    fn book(&self) -> &LazyHash<FontBook> {
        &self.book
    }

    fn main(&self) -> FileId {
        self.main
    }

    fn source(&self, id: FileId) -> FileResult<Source> {
        if let Some(hit) = self.sources.lock().unwrap().get(&id) {
            return Ok(hit.clone());
        }
        let raw = self.read(id)?;
        let text = String::from_utf8(raw).map_err(|_| FileError::InvalidUtf8)?;
        let source = Source::new(id, text);
        self.sources.lock().unwrap().insert(id, source.clone());
        Ok(source)
    }

    fn file(&self, id: FileId) -> FileResult<Bytes> {
        if let Some(hit) = self.files.lock().unwrap().get(&id) {
            return Ok(hit.clone());
        }
        let bytes = Bytes::new(self.read(id)?);
        self.files.lock().unwrap().insert(id, bytes.clone());
        Ok(bytes)
    }

    fn font(&self, index: usize) -> Option<Font> {
        self.fonts.get(index).cloned()
    }

    fn today(&self, _offset: Option<i64>) -> Option<Datetime> {
        self.today
    }
}

/// Load every font file under the given folders, in a stable order.
///
/// Order matters: `FontBook` indices are what `World::font` is asked for, and a
/// different order changes which face wins a tie.
fn load_fonts(dirs: &[PathBuf]) -> Vec<Font> {
    let mut paths: Vec<PathBuf> = Vec::new();
    for dir in dirs {
        collect(dir, &mut paths);
    }
    paths.sort();

    let mut fonts = Vec::new();
    for path in paths {
        let Ok(raw) = std::fs::read(&path) else {
            continue;
        };
        let bytes = Bytes::new(raw);
        for font in Font::iter(bytes) {
            fonts.push(font);
        }
    }

    // typst's own embedded faces go last, so a folder font wins a name tie —
    // the order typst-cli's FontSearcher uses. These carry New Computer Modern,
    // which the sb2nov theme asks for and `rendercv_fonts` does not ship.
    for raw in typst_assets::fonts() {
        for font in Font::iter(Bytes::new(raw)) {
            fonts.push(font);
        }
    }

    fonts
}

fn collect(dir: &Path, out: &mut Vec<PathBuf>) {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            collect(&path, out);
            continue;
        }
        let ext = path
            .extension()
            .and_then(|e| e.to_str())
            .unwrap_or_default()
            .to_ascii_lowercase();
        if matches!(ext.as_str(), "ttf" | "otf" | "ttc" | "otc") {
            out.push(path);
        }
    }
}

struct Args {
    root: PathBuf,
    package_root: PathBuf,
    font_dirs: Vec<PathBuf>,
    input: PathBuf,
    output: PathBuf,
    format: String,
    ppi: f32,
    today: Option<Datetime>,
}

fn parse_args() -> Result<Args, String> {
    let mut root = PathBuf::from("/work");
    let mut package_root = PathBuf::from("/pkg");
    let mut font_dirs = Vec::new();
    let mut input = None;
    let mut output = None;
    let mut format = "pdf".to_string();
    let mut ppi = 144.0f32;
    let mut today = None;

    let argv: Vec<String> = std::env::args().collect();
    let mut i = 1;
    while i < argv.len() {
        let flag = argv[i].as_str();
        let value = || -> Result<String, String> {
            argv.get(i + 1)
                .cloned()
                .ok_or_else(|| format!("{flag} needs a value"))
        };
        match flag {
            "--root" => root = PathBuf::from(value()?),
            "--pkg" => package_root = PathBuf::from(value()?),
            "--font-dir" => font_dirs.push(PathBuf::from(value()?)),
            "--in" => input = Some(PathBuf::from(value()?)),
            "--out" => output = Some(PathBuf::from(value()?)),
            "--format" => format = value()?,
            "--ppi" => ppi = value()?.parse().map_err(|_| "bad --ppi".to_string())?,
            "--today" => {
                let raw = value()?;
                let parts: Vec<&str> = raw.split('-').collect();
                if parts.len() != 3 {
                    return Err("--today wants YYYY-MM-DD".into());
                }
                today = Datetime::from_ymd(
                    parts[0].parse().map_err(|_| "bad year".to_string())?,
                    parts[1].parse().map_err(|_| "bad month".to_string())?,
                    parts[2].parse().map_err(|_| "bad day".to_string())?,
                );
            }
            other => return Err(format!("unknown flag {other}")),
        }
        i += 2;
    }

    Ok(Args {
        root,
        package_root,
        font_dirs,
        input: input.ok_or("--in is required")?,
        output: output.ok_or("--out is required")?,
        format,
        ppi,
        today,
    })
}

fn main() {
    if let Err(err) = run() {
        eprintln!("{err}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let args = parse_args()?;

    let fonts = load_fonts(&args.font_dirs);
    if fonts.is_empty() {
        return Err("no fonts found in the given --font-dir folders".into());
    }
    let book = FontBook::from_fonts(&fonts);

    // The main file is addressed relative to the root, which is what makes the
    // photo's base-name reference resolve (`pdf_png.py:98-111`).
    let rel = args
        .input
        .strip_prefix(&args.root)
        .map_err(|_| "--in must live under --root".to_string())?;
    let main = FileId::new(None, VirtualPath::new(rel));

    let world = ShimWorld {
        library: LazyHash::new(Library::builder().build()),
        book: LazyHash::new(book),
        fonts,
        root: args.root.clone(),
        package_root: args.package_root.clone(),
        main,
        sources: Mutex::new(HashMap::new()),
        files: Mutex::new(HashMap::new()),
        today: args.today,
    };

    let compiled = typst::compile::<PagedDocument>(&world);
    let document = match compiled.output {
        Ok(document) => document,
        Err(errors) => return Err(render_diagnostics(&errors)),
    };

    match args.format.as_str() {
        "pdf" => {
            let options = typst_pdf::PdfOptions {
                ident: Smart::Auto,
                ..Default::default()
            };
            let bytes: SourceResult<Vec<u8>> = typst_pdf::pdf(&document, &options);
            let bytes = bytes.map_err(|errors| render_diagnostics(&errors))?;
            std::fs::write(&args.output, bytes).map_err(|e| e.to_string())?;
            println!("pages {}", document.pages.len());
        }
        "png" => {
            let scale = args.ppi / 72.0;
            for (index, page) in document.pages.iter().enumerate() {
                let pixmap = typst_render::render(page, scale);
                let png = pixmap.encode_png().map_err(|e| e.to_string())?;
                // Upstream names pages `<stem>_<n>.png`, one-based
                // (`pdf_png.py:86-89`).
                let name = format!(
                    "{}_{}.png",
                    args.output.to_string_lossy(),
                    index + 1
                );
                std::fs::write(&name, png).map_err(|e| e.to_string())?;
            }
            println!("pages {}", document.pages.len());
        }
        other => return Err(format!("unknown --format {other}")),
    }

    Ok(())
}

fn render_diagnostics(errors: &[typst::diag::SourceDiagnostic]) -> String {
    errors
        .iter()
        .map(|d| d.message.to_string())
        .collect::<Vec<_>>()
        .join("\n")
}
