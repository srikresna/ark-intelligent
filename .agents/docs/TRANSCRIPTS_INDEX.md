# Transcripts Index

## Purpose
This directory contains processed transcripts from YouTube videos, playlists, and PDFs. All content is structured for easy indexing and retrieval by LLM agents.

## Directory Structure
```
docs/transcripts/
├── youtube/
│   ├── {video_id}/
│   │   ├── metadata.json
│   │   ├── transcript.md
│   │   ├── summary.md
│   │   └── diagrams/
│   │       ├── flowchart.md
│   │       └── mindmap.md
│   └── {playlist_id}/
│       ├── metadata.json
│       ├── combined_summary.md
│       └── videos/
│           └── {video_id}/
└── pdf/
    └── {filename_slug}/
        ├── metadata.json
        ├── full_text.md
        ├── summary.md
        └── diagrams/
```

## File Formats

### metadata.json
```json
{
  "source_type": "youtube|pdf",
  "source_id": "video_id or filename",
  "title": "Document Title",
  "created_at": "ISO8601 timestamp",
  "language": "id|en",
  "duration_seconds": 1234,
  "page_count": 50,
  "tags": ["trading", "strategy", "risk-management"],
  "processing_notes": "OCR used for 20 pages"
}
```

### transcript.md / full_text.md
- Clean Markdown format
- Section headers preserved
- Tables converted to Markdown tables
- Page breaks marked with `---`

### summary.md
- Executive summary
- Key points bulleted
- Actionable insights
- Tags for search

### diagrams/*.md
- Mermaid.js source code
- Rendered as code blocks
- LLM-readable

## Search Guidelines
Agents should:
1. Read `TRANSCRIPTS_INDEX.md` first
2. Use metadata.json for quick filtering
3. Read summary.md for overview
4. Access full_text.md for detailed analysis
5. Reference diagrams for visual concepts
