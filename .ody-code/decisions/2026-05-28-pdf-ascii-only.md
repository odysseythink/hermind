# create-files-agent — PDF ASCII-Only in v3-B3

**Date**: 2026-05-28
**Status**: Adopted
**Context**: go-pdf/fpdf defaults to Helvetica which only supports Latin-1. CJK/Arabic characters require TTF font registration.

**Decision**: v3-B3 detects non-ASCII characters and returns `tool.Error("PDF generation does not yet support non-ASCII text; please use markdown instead")`.

**Reconsider when**: User explicitly needs CJK PDF; follow-up PR to add TTF font registration (may need to ship an open-source CJK font file).
