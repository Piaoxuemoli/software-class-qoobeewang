#!/usr/bin/env bash
# build-docs.sh — docs -> PDF pipeline.
#
#   1. Render every docs/**/diagrams/*.puml to a same-name .png (committed).
#   2. For each line of docs/pdf-manifest.tsv: pandoc (gfm -> docx, using
#      docs/templates/reference.docx) -> soffice --headless -> PDF into
#      ${DOCS_OUT:-docs/pdf}/.
#
# Cover block (title / subtitle / author / date) comes from pandoc metadata;
# TODO: matching the official cover template (course/06-提交模板) is a later
# task.
#
# Environment:
#   DOCS_PROFILE  pandoc metadata profile yaml.
#                 Default: docs/deliverables/profile.example.yaml (placeholders).
#   DOCS_OUT      output dir. Default: docs/pdf.
#                 When DOCS_PROFILE is set, DOCS_OUT is REQUIRED and must be
#                 outside the repo OR git-ignored (e.g. course/07-提交物) so
#                 real names / student IDs can never be committed.
#   PLANTUML_JAR  optional path to a plantuml.jar (uses `java -jar`);
#                 otherwise the `plantuml` command must be on PATH.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"

BUILD="$ROOT/build/docs"
OUT=${DOCS_OUT:-$ROOT/docs/pdf}
PROFILE=${DOCS_PROFILE:-$ROOT/docs/deliverables/profile.example.yaml}
MANIFEST="$ROOT/docs/pdf-manifest.tsv"
REFDOC="$ROOT/docs/templates/reference.docx"
mkdir -p "$BUILD" "$OUT"

die() { echo "build-docs: ERROR: $*" >&2; exit 1; }
warn() { echo "build-docs: WARNING: $*" >&2; }

[ -f "$PROFILE" ] || die "profile not found: $PROFILE"
[ -f "$REFDOC" ] || die "reference doc not found: $REFDOC (run scripts/docs/make-reference-docx.py)"
[ -f "$MANIFEST" ] || die "manifest not found: $MANIFEST"

if [ -n "${DOCS_PROFILE:-}" ]; then
  [ -n "${DOCS_OUT:-}" ] || die "DOCS_PROFILE is set: DOCS_OUT is required (outside the repo or git-ignored)"
  abs_out=$(realpath -m "$OUT")
  case "$abs_out" in
    "$ROOT"|"$ROOT"/*)
      if command -v git >/dev/null 2>&1; then
        if ! git -C "$ROOT" check-ignore -q "${abs_out#"$ROOT"/}"; then
          die "DOCS_OUT=$abs_out is inside the repo and NOT git-ignored; refusing (identity leak risk)"
        fi
      else
        # No git in this environment: only course/ (always gitignored) is safe.
        case "${abs_out#"$ROOT"/}" in
          course/*) ;;
          *) die "DOCS_OUT=$abs_out is inside the repo and git is unavailable to verify ignore status; refusing" ;;
        esac
      fi
      ;;
  esac
fi

# PlantUML runner
if [ -n "${PLANTUML_JAR:-}" ]; then
  puml() { java -jar "$PLANTUML_JAR" "$@"; }
else
  command -v plantuml >/dev/null || die "plantuml not found (set PLANTUML_JAR or use docker/docs image)"
  puml() { plantuml "$@"; }
fi

# --- 1. diagrams ------------------------------------------------------------
echo "== rendering diagrams =="
found_puml=0
while IFS= read -r -d '' f; do
  found_puml=1
  echo "  $f"
  puml -tpng "$f" || die "plantuml failed on $f"
done < <(find docs -type f -name '*.puml' -path '*/diagrams/*' -print0)
[ "$found_puml" = 1 ] || echo "  (no .puml files)"

# --- helpers ----------------------------------------------------------------
# Read a scalar key from the profile yaml (strips quotes and comments).
yget() {
  sed -n "s/^[[:space:]]*$1[[:space:]]*:[[:space:]]*//p" "$PROFILE" \
    | head -1 | sed -e 's/[[:space:]]*#.*$//' -e 's/^"//' -e 's/"$//' \
      -e "s/^'//" -e "s/'\$//" -e 's/[[:space:]]*$//'
}

pagebreak=$'```{=openxml}\n<w:p><w:r><w:br w:type="page"/></w:r></w:p>\n```'

# --- 2. documents -----------------------------------------------------------
echo "== building PDFs (profile: $PROFILE) =="
export HOME="${HOME:-/tmp}"

while IFS=$'\t' read -r outname rest; do
  [[ $outname =~ ^[[:space:]]*(#|$) ]] && continue
  # collect existing sources
  declare -a srcs=()
  for s in $rest; do
    if [ -f "$ROOT/$s" ]; then srcs+=("$s"); else warn "missing source $s (for $outname)"; fi
  done
  if [ "${#srcs[@]}" -eq 0 ]; then
    warn "all sources missing for $outname — skipping"
    continue
  fi

  stem=${outname%.pdf}
  # Document title: output name without extension and leading `NN-` prefix.
  title=$(printf '%s' "$stem" | sed 's/^[0-9][0-9]*-//')

  # Per-doc metadata: profile (minus title/subtitle/author/student-id) +
  # document title + subtitle + author lines (组长 / 学号) + Chinese TOC title.
  metayaml="$BUILD/$stem.meta.yaml"
  grep -vE '^(title|subtitle|author|student-id)[[:space:]]*:' "$PROFILE" > "$metayaml"
  {
    printf 'title: "%s"\n' "$title"
    printf 'subtitle: "数据备份软件（DataBackup）"\n'
    printf 'toc-title: "目录"\n'
    printf 'author:\n'
    a=$(yget author); [ -n "$a" ] && printf '  - "%s"\n' "$a"
    sid=$(yget student-id); [ -n "$sid" ] && printf '  - "%s"\n' "$sid"
  } >> "$metayaml"

  docx="$BUILD/$stem.docx"
  if [ "${#srcs[@]}" -gt 1 ]; then
    # Combined document: sources joined by page breaks; the cover block
    # (title/subtitle/author/date) comes from pandoc metadata.
    tmpmd="$BUILD/$stem.combined.md"
    {
      first=1
      for s in "${srcs[@]}"; do
        [ "$first" = 1 ] || printf '%s\n\n' "$pagebreak"
        first=0
        cat "$ROOT/$s"
        printf '\n'
      done
    } > "$tmpmd"
    inputs=("$tmpmd")
  else
    inputs=("$ROOT/${srcs[0]}")
  fi

  # resource path: every source dir + repo root (for images/links)
  rp="$ROOT"
  for s in "${srcs[@]}"; do rp="$rp:$(dirname "$ROOT/$s")"; done

  echo "  $outname <- ${srcs[*]}"
  pandoc --from=gfm+raw_attribute --to=docx \
    --resource-path="$rp" \
    --metadata-file="$metayaml" \
    --reference-doc="$REFDOC" \
    --toc --toc-depth=3 \
    -o "$docx" "${inputs[@]}" || die "pandoc failed for $outname"
  pdf="$BUILD/$stem.pdf"
  python3 "$ROOT/scripts/docs/docx2pdf.py" "$docx" "$pdf" \
    || die "docx2pdf failed for $outname"
  mv -f "$pdf" "$OUT/$outname"
  echo "    -> $OUT/$outname"
  unset srcs
done < "$MANIFEST"

echo "build-docs: done. Output in $OUT"
