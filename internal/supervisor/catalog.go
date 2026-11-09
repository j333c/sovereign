package supervisor

// DefaultServiceCatalog returns the canonical service definitions for the appliance.
func DefaultServiceCatalog() map[string]ServiceDef {
	return map[string]ServiceDef{
		ServiceJellyfin: {
			ID:                  ServiceJellyfin,
			Name:                "Jellyfin",
			Description:         "Video Streaming",
			Image:               ImageJellyfin,
			MemLimitBytes:       MemLimitJellyfin,
			MemReservationBytes: MemReservationJellyfin,
			Port:                8096,
			Subpath:             "/media",
			VolumeTemplates: []string{
				"media/movies:/data/media/movies",
				"media/tv:/data/media/tv",
				"state/services/jellyfin/config:/config",
				"state/services/jellyfin/cache:/cache",
			},
		},
		ServiceNavidrome: {
			ID:                  ServiceNavidrome,
			Name:                "Navidrome",
			Description:         "Music Streaming",
			Image:               ImageNavidrome,
			MemLimitBytes:       MemLimitNavidrome,
			MemReservationBytes: MemReservationNavidrome,
			Port:                4533,
			Subpath:             "/music",
			VolumeTemplates: []string{
				"media/music:/music:ro",
				"state/services/navidrome/data:/data",
			},
		},
		ServiceAudiobookshelf: {
			ID:                  ServiceAudiobookshelf,
			Name:                "Audiobookshelf",
			Description:         "Audiobook Streaming",
			Image:               ImageAudiobookshelf,
			MemLimitBytes:       MemLimitAudiobookshelf,
			MemReservationBytes: MemReservationAudiobookshelf,
			Port:                13378,
			Subpath:             "/audiobooks",
			VolumeTemplates: []string{
				"media/audiobooks:/audiobooks",
				"state/services/audiobookshelf/config:/config",
				"state/services/audiobookshelf/metadata:/metadata",
			},
		},
		ServicePaperless: {
			ID:                  ServicePaperless,
			Name:                "Paperless-ngx",
			Description:         "Document Management",
			Image:               ImagePaperless,
			MemLimitBytes:       MemLimitPaperless,
			MemReservationBytes: MemReservationPaperless,
			Port:                8000,
			Subpath:             "/docs",
			VolumeTemplates: []string{
				"documents/data:/usr/src/paperless/data",
				"documents/media:/usr/src/paperless/media",
				"documents/consume:/usr/src/paperless/consume",
				"documents/export:/usr/src/paperless/export",
				"state/services/paperless:/usr/src/paperless/state",
			},
		},
		ServiceLlama: {
			ID:                  ServiceLlama,
			Name:                "llama.cpp",
			Description:         "Local AI Inference",
			Image:               ImageLlama,
			MemLimitBytes:       MemLimitLlama,
			MemReservationBytes: MemReservationLlama,
			Port:                DefaultLlamaPort,
			Subpath:             "/v1",
			VolumeTemplates: []string{
				"models:/models:ro",
			},
		},
		ServiceOpenWebUI: {
			ID:                  ServiceOpenWebUI,
			Name:                "Open WebUI",
			Description:         "Private AI Chat",
			Image:               ImageOpenWebUI,
			MemLimitBytes:       MemLimitOpenWebUI,
			MemReservationBytes: MemReservationOpenWebUI,
			Port:                DefaultOpenWebUIPort,
			Subpath:             DefaultOpenWebUISubpath,
			VolumeTemplates: []string{
				"state/services/openwebui:/app/backend/data",
			},
		},
	}
}
