package tracks

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	basemath "github.com/antonybholmes/go-basemath"
	"github.com/antonybholmes/go-dna"
	"github.com/antonybholmes/go-sys"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

// const MAGIC_NUMBER_OFFSET_BYTES = 0
// const BIN_SIZE_OFFSET_BYTES = MAGIC_NUMBER_OFFSET_BYTES + 4
// const BIN_WIDTH_OFFSET_BYTES = BIN_SIZE_OFFSET_BYTES + 4
// const N_BINS_OFFSET_BYTES = BIN_WIDTH_OFFSET_BYTES + 4
// const BINS_OFFSET_BYTES = N_BINS_OFFSET_BYTES + 4

const TRACK_SQL = `SELECT public_id, name, reads, stat_mode FROM track`

const FIND_TRACK_SQL = `SELECT platform, genome, name, reads, stat_mode, dir FROM tracks WHERE tracks.public_id = ?1`

const BIN_SQL = `SELECT start, end, reads 
	FROM bins
 	WHERE start >= ?1 AND end < ?2
	ORDER BY start`

type BinCounts struct {
	Track    Track         `json:"track"`
	Location *dna.Location `json:"location"`
	Bins     []uint        `json:"bins"`
	Start    uint          `json:"start"`
	BinWidth uint          `json:"binWidth"`
}

type Track struct {
	Platform string `json:"platform"`
	Genome   string `json:"genome"`
	Name     string `json:"name"`
}

type TrackInfo struct {
	PublicId string `json:"publicId"`
	Platform string `json:"platform"`
	Genome   string `json:"genome"`
	Name     string `json:"name"`
	Stat     string `json:"stat"`
	Reads    uint   `json:"reads"`
}

type TrackGenome struct {
	Name   string      `json:"name"`
	Tracks []TrackInfo `json:"tracks"`
}

type TrackPlaform struct {
	Name    string        `json:"name"`
	Genomes []TrackGenome `json:"genomes"`
}

type AllTracks struct {
	Name      string         `json:"name"`
	Platforms []TrackPlaform `json:"platforms"`
}

type TrackReader struct {
	Dir      string
	Stat     string
	Track    Track
	BinWidth uint
	Reads    uint
}

func NewTrackReader(dir string, track Track, binWidth uint) (*TrackReader, error) {

	dir = filepath.Join(dir, track.Platform, track.Genome, track.Name)

	path := filepath.Join(dir, "track.db")

	db, err := sql.Open("sqlite3", path)

	if err != nil {
		return nil, err
	}

	defer db.Close()

	var reads uint
	var name string
	var publicId string
	var stat string
	err = db.QueryRow(TRACK_SQL).Scan(&publicId, &name, &reads, &stat)

	if err != nil {
		return nil, err
	}

	// if err != nil {
	// 	return nil, fmt.Errorf("error opening %s", file)
	// }

	// defer file.Close()
	// // Create a scanner
	// scanner := bufio.NewScanner(file)
	// scanner.Scan()

	// count, err := strconv.Atoi(scanner.Text())

	// if err != nil {
	// 	return nil, fmt.Errorf("could not count reads")
	// }

	return &TrackReader{Dir: dir,
		Stat:     stat,
		BinWidth: binWidth,
		Reads:    reads,
		Track:    track}, nil
}

func (reader *TrackReader) getPath(location *dna.Location) string {
	return filepath.Join(reader.Dir, fmt.Sprintf("%s_bw%d_%s.db", strings.ToLower(location.Chr), reader.BinWidth, reader.Track.Genome))

}

func (reader *TrackReader) BinCounts(location *dna.Location) (*BinCounts, error) {

	path := reader.getPath(location)

	log.Debug().Msgf("track path %s", path)

	db, err := sql.Open("sqlite3", path)

	if err != nil {
		log.Debug().Msgf("bin sql err %s", err)
		return nil, err
	}

	defer db.Close()

	startBin := (location.Start - 1) / reader.BinWidth
	endBin := (location.End - 1) / reader.BinWidth

	rows, err := db.Query(BIN_SQL,
		startBin,
		endBin)

	if err != nil {
		return nil, err
	}

	var readBlockStart uint
	var readBlockEnd uint
	var count uint
	reads := make([]uint, endBin-startBin+1)
	index := 0

	for rows.Next() {
		err := rows.Scan(&readBlockStart, &readBlockEnd, &count)

		if err != nil {
			return nil, err //fmt.Errorf("there was an error with the database records")
		}

		// we don't want to load bin data that goes outside our coordinates
		// of interest. A long gapped bin, may end beyond the blocks we are
		// interested in, so we need to stop the loop short if so.
		endBin := basemath.UintMin(startBin+uint(len(reads)), readBlockEnd)

		for bin := readBlockStart; bin < endBin; bin++ {
			reads[bin-startBin] = count
		}

		index++
	}

	return &BinCounts{
		Track:    reader.Track,
		Location: location,
		Start:    startBin*reader.BinWidth + 1,
		Bins:     reads,

		BinWidth: reader.BinWidth,
	}, nil

	// var magic uint32
	// binary.Read(f, binary.LittleEndian, &magic)
	// var binSizeBytes byte
	// binary.Read(f, binary.LittleEndian, &binSizeBytes)

	// switch binSizeBytes {
	// case 1:
	// 	return reader.ReadsUint8(location)
	// case 2:
	// 	return reader.ReadsUint16(location)
	// default:
	// 	return reader.ReadsUint32(location)
	// }
}

// func (reader *TracksReader) ReadsUint8(location *dna.Location) (*BinCounts, error) {
// 	s := location.Start - 1
// 	e := location.End - 1

// 	bs := s / reader.BinWidth
// 	be := e / reader.BinWidth
// 	bl := be - bs + 1

// 	file := reader.getPath(location)

// 	f, err := os.Open(file)

// 	if err != nil {
// 		return nil, err
// 	}

// 	defer f.Close()

// 	//var magic uint32
// 	//binary.Read(f, binary.LittleEndian, &magic)

// 	f.Seek(9, 0)

// 	offset := BINS_OFFSET_BYTES + bs
// 	log.Debug().Msgf("offset %d %d", offset, bs)

// 	data := make([]uint8, bl)
// 	f.Seek(int64(offset), 0)
// 	binary.Read(f, binary.LittleEndian, &data)

// 	reads := make([]uint32, bl)

// 	for i, c := range data {
// 		reads[i] = uint32(c)
// 	}

// 	return reader.Results(location, bs, reads)
// }

// func (reader *TracksReader) ReadsUint16(location *dna.Location) (*BinCounts, error) {
// 	s := location.Start - 1
// 	e := location.End - 1

// 	bs := s / reader.BinWidth
// 	be := e / reader.BinWidth
// 	bl := be - bs + 1

// 	file := reader.getPath(location)

// 	f, err := os.Open(file)

// 	if err != nil {
// 		return nil, err
// 	}

// 	defer f.Close()

// 	f.Seek(9, 0)

// 	data := make([]uint16, bl)
// 	f.Seek(int64(BINS_OFFSET_BYTES+bs*2), 0)
// 	binary.Read(f, binary.LittleEndian, &data)

// 	reads := make([]uint32, bl)

// 	for i, c := range data {
// 		reads[i] = uint32(c)
// 	}

// 	return reader.Results(location, bs, reads)
// }

// func (reader *TracksReader) ReadsUint32(location *dna.Location) (*BinCounts, error) {
// 	s := location.Start - 1
// 	e := location.End - 1

// 	bs := s / reader.BinWidth
// 	be := e / reader.BinWidth
// 	bl := be - bs + 1

// 	file := reader.getPath(location)

// 	f, err := os.Open(file)

// 	if err != nil {
// 		return nil, err
// 	}

// 	defer f.Close()

// 	f.Seek(9, 0)

// 	reads := make([]uint32, bl)
// 	f.Seek(int64(BINS_OFFSET_BYTES+bs*4), 0)
// 	binary.Read(f, binary.LittleEndian, &reads)

// 	return reader.Results(location, bs, reads)
// }

// func (reader *TracksReader) Results(location *dna.Location, bs uint, reads []uint32) (*BinCounts, error) {

// 	return &BinCounts{
// 		Location: location,
// 		Start:    bs*reader.BinWidth + 1,
// 		Reads:    reads,
// 		ReadN:    reader.ReadN,
// 	}, nil
// }

type TracksDB struct {
	cacheMap map[string]map[string][]TrackInfo
	db       *sql.DB
	dir      string
}

func (tracksDb *TracksDB) Dir() string {
	return tracksDb.dir
}

func NewTrackDB(dir string) *TracksDB {
	cacheMap := make(map[string]map[string][]TrackInfo)

	platformFiles, err := os.ReadDir(dir)

	log.Debug().Msgf("---- track db ----")

	if err != nil {
		log.Fatal().Msgf("error opening %s", dir)
	}

	log.Debug().Msgf("caching track databases in %s...", dir)

	// Sort by name
	sort.Slice(platformFiles, func(i, j int) bool {
		return platformFiles[i].Name() < platformFiles[j].Name()
	})

	for _, platform := range platformFiles {
		if platform.IsDir() {

			log.Debug().Msgf("found platform %s", platform.Name())

			cacheMap[platform.Name()] = make(map[string][]TrackInfo)

			platformDir := filepath.Join(dir, platform.Name())

			genomeFiles, err := os.ReadDir(platformDir)

			if err != nil {
				log.Fatal().Msgf("error opening %s", platformDir)
			}

			sort.Slice(genomeFiles, func(i, j int) bool {
				return genomeFiles[i].Name() < genomeFiles[j].Name()
			})

			for _, genome := range genomeFiles {
				if genome.IsDir() {

					log.Debug().Msgf("found genome %s", genome.Name())

					sampleDir := filepath.Join(dir, platform.Name(), genome.Name())

					sampleFiles, err := os.ReadDir(sampleDir)

					if err != nil {
						log.Fatal().Msgf("error sample dir %s", sampleDir)
					}

					cacheMap[platform.Name()][genome.Name()] = make([]TrackInfo, 0, 10)

					// Sort by name
					sort.Slice(sampleFiles, func(i, j int) bool {
						return sampleFiles[i].Name() < sampleFiles[j].Name()
					})

					for _, sample := range sampleFiles {
						if sample.IsDir() {
							log.Debug().Msgf("found sample %s", sample.Name())

							path := filepath.Join(dir, platform.Name(), genome.Name(), sample.Name(), "track.db")

							db := sys.Must(sql.Open("sqlite3", path))

							defer db.Close()

							var reads uint
							var name string
							var publicId string
							var stat string
							err := db.QueryRow(TRACK_SQL).Scan(&publicId, &name, &reads, &stat)

							if err != nil {
								log.Fatal().Msgf("info not found %s", err)
							}

							cacheMap[platform.Name()][genome.Name()] = append(cacheMap[platform.Name()][genome.Name()], TrackInfo{Platform: platform.Name(),
								Genome:   genome.Name(),
								PublicId: publicId,
								Name:     name,
								Reads:    reads,
								Stat:     stat})
						}
					}
				}
			}
		}
	}

	log.Debug().Msgf("%v", cacheMap)

	log.Debug().Msgf("---- end ----")

	db := sys.Must(sql.Open("sqlite3", filepath.Join(dir, "tracks.db")))

	return &TracksDB{dir: dir, cacheMap: cacheMap, db: db}
}

func (tracksDb *TracksDB) Platforms() []string {
	keys := make([]string, 0, len(tracksDb.cacheMap))

	for k := range tracksDb.cacheMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func (tracksDb *TracksDB) Genomes(platform string) ([]string, error) {
	genomes, ok := tracksDb.cacheMap[platform]

	if ok {
		keys := make([]string, 0, len(genomes))

		for k := range genomes {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		return keys, nil
	} else {
		return nil, fmt.Errorf("platform %s not found", platform)
	}
}

func (tracksDb *TracksDB) Tracks(platform string, genome string) ([]TrackInfo, error) {
	genomes, ok := tracksDb.cacheMap[platform]

	if ok {
		tracks, ok := genomes[genome]

		if ok {
			return tracks, nil
		} else {
			return nil, fmt.Errorf("genome %s not found", genome)
		}
	} else {
		return nil, fmt.Errorf("platform %s not found", platform)
	}
}

func (tracksDb *TracksDB) AllTracks() (*AllTracks, error) {
	platforms := tracksDb.Platforms()

	ret := AllTracks{Name: "Tracks Database", Platforms: make([]TrackPlaform, 0, len(platforms))}

	for _, platform := range platforms {

		genomes, err := tracksDb.Genomes(platform)

		if err != nil {
			return nil, err
		}

		trackPlatform := TrackPlaform{Name: platform, Genomes: make([]TrackGenome, 0, len(genomes))}

		for _, genome := range genomes {
			tracks, err := tracksDb.Tracks(platform, genome)

			if err != nil {
				return nil, err
			}

			trackGenome := TrackGenome{Name: genome, Tracks: tracks}

			trackPlatform.Genomes = append(trackPlatform.Genomes, trackGenome)

		}

		ret.Platforms = append(ret.Platforms, trackPlatform)

	}

	return &ret, nil
}

func (tracksDb *TracksDB) ReaderFromTrackId(publicId string, binWidth uint) (*TrackReader, error) {

	var platform string
	var genome string
	var name string
	var reads uint
	var stat string
	var dir string
	//const FIND_TRACK_SQL = `SELECT platform, genome, name, reads, stat_mode, dir FROM tracks WHERE tracks.publicId = ?1`

	err := tracksDb.db.QueryRow(FIND_TRACK_SQL, publicId).Scan(&platform, &genome, &name, &reads, &stat, &dir)

	if err != nil {
		return nil, err
	}

	track := Track{Platform: platform, Genome: genome, Name: name}

	return NewTrackReader(tracksDb.dir, track, binWidth)
}
