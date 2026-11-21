package vmx

import "github.com/MikanseiLaboratory/vmx-go/internal/tables"

type bitrateConfig struct {
	minQuality     int32
	dcShift        int32
	threads        int32
	targetBytesMin int32
	targetBytesMax int32
}

func calculateBitrate(targetMbps int32, minimum bool) int32 {
	t := float64(targetMbps)
	t /= 60.0 * 8.0
	t *= 1048576.0
	if minimum {
		t *= 0.95
	} else {
		t *= 1.05
	}
	return int32(t)
}

func lookupBitrate(profile Profile, height int) bitrateConfig {
	cfg := bitrateConfig{
		minQuality: 80, dcShift: 0, threads: 2,
		targetBytesMin: calculateBitrate(72, true),
		targetBytesMax: calculateBitrate(72, false),
	}
	for _, row := range tables.BitrateTable {
		if row[0] == int32(profile) && int32(height) >= row[1] {
			cfg.minQuality = row[4]
			cfg.dcShift = row[3]
			cfg.threads = row[5]
			cfg.targetBytesMin = calculateBitrate(row[2], true)
			cfg.targetBytesMax = calculateBitrate(row[2], false)
			break
		}
	}
	return cfg
}

func adjustBitrate(quality *int, minQuality int, frameLen int, targetMin, targetMax int32) {
	if frameLen == 0 || targetMin == 0 || targetMax == 0 {
		return
	}
	q := *quality
	fl := int32(frameLen)
	switch {
	case fl < targetMin:
		if q < minQuality {
			*quality = minQuality
		} else if q < 76 {
			*quality = min(q+4, MaxQ)
		} else if q < 92 {
			*quality = min(q+2, MaxQ)
		} else if q < 99 {
			*quality = min(q+1, MaxQ)
		}
	case fl > targetMax:
		if q > 92 {
			*quality = q - 1
		} else if q > minQuality {
			*quality = max(q-2, minQuality)
		} else {
			*quality = minQuality
		}
	}
}
