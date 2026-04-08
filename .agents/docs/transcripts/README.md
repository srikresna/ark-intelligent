# Transcripts Directory

This directory will contain processed transcripts from:
- YouTube videos and playlists
- PDF documents

## Structure
```
transcripts/
├── youtube/
│   ├── {video_id}/
│   └── {playlist_id}/
└── pdf/
    └── {filename_slug}/
```

## Status
⏳ Waiting for first content to be processed...

Once content is processed by ark-transcriber, it will be automatically saved here with:
- metadata.json (structured data)
- summary.md (AI summary + key points)
- transcript.md / full_text.md (full content)
- diagrams/ (Mermaid.js visualizations)

All files are optimized for LLM indexing and search.
