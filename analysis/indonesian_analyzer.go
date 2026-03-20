// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// IndonesianStopWords contains common Indonesian stop words.
// Source: Apache Lucene Indonesian (Bahasa) stop words list
var IndonesianStopWords = []string{
	"ada", "adalah", "adanya", "adapun", "agak", "agaknya", "agar", "akan",
	"akankah", "akhir", "akhiri", "akhirnya", "aku", "akulah", "amat",
	"amatlah", "anda", "andalah", "antar", "antara", "antaranya", "apa",
	"apaan", "apabila", "apakah", "apalagi", "apatah", "artinya", "asal",
	"asalkan", "atas", "atau", "ataukah", "ataupun", "awal", "awalnya",
	"bagai", "bagaikan", "bagaimana", "bagaimanakah", "bagaimanapun", "bagainamakan",
	"bagi", "bagian", "bahkan", "bahwa", "bahwasannya", "bahwasanya",
	"baik", "baiklah", "bakal", "bakalan", "balik", "banyak", "bapak",
	"baru", "bawah", "beberapa", "begini", "beginian", "beginikah",
	"beginilah", "begitu", "begitukah", "begitulah", "begitupun",
	"bekerja", "belakang", "belakangan", "belum", "belumlah", "benar",
	"benarkah", "benarlah", "berada", "berakhir", "berapa", "berapakah",
	"berapalah", "berapapun", "berarti", "berawal", "berbagai",
	"berdatangan", "beri", "berikan", "berikut", "berikutnya",
	"berjumlah", "berkali-kali", "berkata", "berkehendak", "berkeinginan",
	"berkenaan", "berlainan", "berlalu", "berlangsung", "berlebihan",
	"bermacam", "bermacam-macam", "bermaksud", "bermula", "bersama",
	"bersama-sama", "bersiap", "bertanya", "bertanya-tanya", "berturut",
	"berturut-turut", "bertutur", "berujar", "berupa", "besar",
	"betul", "betulkah", "biasa", "biasanya", "bila", "bilakah",
	"bisa", "bisakah", "boleh", "bolehkah", "bolehlah", "buat",
	"bukan", "bukankah", "bukanlah", "bukannya", "cukup", "cukupkah",
	"cukuplah", "cuma", "dahulu", "dalam", "dan", "dapat", "dari",
	"daripada", "datang", "dekat", "demi", "demikian", "demikianlah",
	"dengan", "depan", "di", "dia", "diakhiri", "diakhirinya",
	"dialah", "diantara", "diantaranya", "diberi", "diberikan",
	"diberikannya", "dibuat", "dibuatnya", "didapat", "didatangkan",
	"digunakan", "diibaratkan", "diibaratkannya", "diingat",
	"diingatkan", "diinginkan", "dijawab", "dijelaskan", "dijelaskannya",
	"dimaksud", "dimaksudkan", "dimaksudkannya", "dimasukkan",
	"diminta", "dimintai", "dimisalkan", "dimulai", "dimulailah",
	"dimulainya", "dimungkinkan", "dini", "dipastikan", "diperbuat",
	"diperbuatnya", "dipergunakan", "diperkirakan", "diperlihatkan",
	"diperlukan", "diperlukannya", "dipersoalkan", "dipertanyakan",
	"dipergunakan", "dipunyai", "diri", "dirinya", "disampaikan",
	"disebut", "disebutkan", "disebutkannya", "disini", "disinilah",
	"ditambahkan", "ditandaskan", "ditanya", "ditanyai", "ditanyakan",
	"ditegaskan", "ditemukan", "ditiadakan", "ditujukan", "ditunjuk",
	"ditunjuki", "ditunjukkan", "ditunjukkannya", "dituruti", "diucapkan",
	"diucapkannya", "diungkapkan", "dong", "dua", "dulu", "empat",
	"enggak", "enggaknya", "entah", "entahlah", "guna", "gunakan",
	"hal", "hampir", "hanya", "hanyalah", "harus", "haruslah",
	"harusnya", "hendak", "hendaklah", "hendaknya", "hingga",
	"ia", "ialah", "ibarat", "ibaratkan", "ibaratnya", "ikut",
	"ingat", "ingat-ingat", "ingin", "inginkah", "inginkan",
	"ini", "inikah", "inilah", "itu", "itukah", "itulah",
	"jadi", "jadilah", "jadinya", "jangan", "jangankan", "janganlah",
	"jauh", "jawab", "jawaban", "jawabnya", "jelas", "jelaskan",
	"jelaslah", "jelasnya", "jika", "jikalau", "juga", "jumlah",
	"jumlahnya", "justru", "kala", "kalau", "kalaulah", "kalaupun",
	"kalian", "kami", "kamilah", "kamu", "kamulah", "kan", "kapan",
	"kapankah", "kapanpun", "karena", "karenanya", "kasus", "kata",
	"katakan", "katakanlah", "katanya", "ke", "keadaan", "kebetulan",
	"kecil", "kedua", "keluar", "kelompok", "kembali", "kemudian",
	"kemungkinan", "kemungkinannya", "kenapa", "kepada", "kepadanya",
	"kesampaian", "keseluruhan", "keseluruhannya", "keterlaluan",
	"ketika", "khususnya", "kini", "kinilah", "kiranya", "kira-kira",
	"kitalah", "kurang", "lagi", "lagian", "lah", "lain",
	"lainnya", "lalu", "lama", "lamanya", "lanjut", "lanjutnya",
	"lebih", "lewat", "lima", "luar", "macam", "maka",
	"makanya", "makin", "malah", "malahan", "mampu", "mampukah",
	"mana", "manakala", "manalagi", "masa", "masalah", "masalahnya",
	"masih", "masihkah", "masing", "masing-masing", "mau", "maupun",
	"melainkan", "melakukan", "melalui", "melihat", "melihatnya",
	"memang", "memastikan", "memberi", "memberikan", "membuat",
	"memerlukan", "memihak", "meminta", "memintakan", "memisalkan",
	"memperbuat", "mempergunakan", "memperkirakan", "memperlihatkan",
	"mempersiapkan", "mempersoalkan", "mempertanyakan", "mempunyai",
	"memulai", "memungkinkan", "menaiki", "menambahkan", "menandaskan",
	"menanti", "menantikan", "menanya", "menanyai", "menanyakan",
	"mendapat", "mendapatkan", "mendatang", "mendatangi", "mendatangkan",
	"menegaskan", "menerima", "meneruskan", "mengadakan", "mengatakan",
	"mengatakannya", "mengenai", "mengerjakan", "mengetahui",
	"menggunakan", "menghendaki", "mengibaratkan", "mengibaratkannya",
	"mengingat", "mengingatkan", "menginginkan", "mengingkari",
	"mengirimkan", "mengucapkan", "mengucapkannya", "mengungkapkan",
	"menjadi", "menjawab", "menjelaskan", "menjelaskannya", "menunjuk",
	"menunjuki", "menunjukkan", "menunjukkannya", "menurut",
	"menuturkan", "menyampaikan", "menyangkut", "menyebutkan",
	"menyediakan", "merasa", "mereka", "merekalah", "merupakan",
	"meski", "meskipun", "meyakini", "meyakinkan", "minta",
	"mirip", "misal", "misalkan", "misalnya", "mula", "mulai",
	"mulailah", "mulanya", "mungkin", "mungkinkah", "nah", "naik",
	"namun", "nanti", "nantinya", "nyaris", "oleh", "olehnya",
	"pada", "padahal", "padanya", "paling", "panjang", "pantas",
	"para", "pasti", "pastilah", "per", "percuma", "perlu",
	"perlukah", "perlunya", "pernah", "pada", "paling",
	"pun", "punya", "rasa", "rasanya", "rata", "rupanya",
	"saat", "saatnya", "saja", "sajalah", "saling", "sama",
	"sama-sama", "sambil", "sampai", "sampai-sampai", "sampaikan",
	"sana", "sangat", "sangatlah", "satu", "saya", "sayalah",
	"se", "sebab", "sebabnya", "sebagai", "sebagaimana",
	"sebagainya", "sebagian", "sebaik", "sebaik-baiknya",
	"sebaiknya", "sebaliknya", "sebanyak", "sebegini", "sebegitu",
	"sebelum", "sebelumnya", "sebenarnya", "seberapa", "sebesar",
	"sebetulnya", "sebisanya", "sebuah", "sebut", "sebutlah",
	"sebutnya", "secara", "sedang", "sedangkan", "sedari",
	"sedemikian", "sedikit", "sedikitnya", "seenaknya", "segala",
	"segalanya", "segera", "sehabis", "seharusnya", "sehingga",
	"seingat", "sejak", "sejauh", "sejenak", "sejumlah", "sekadar",
	"sekadarnya", "sekali", "sekali-kali", "sekalian", "sekaligus",
	"sekalipun", "sekarang", "sekaranglah", "sekecil", "seketika",
	"sekiranya", "sekitar", "sekitarnya", "sekurang-kurangnya",
	"sekurangnya", "selesai", "seluruh", "seluruhnya", "semacam",
	"semakin", "semasih", "semasihnya", "semata", "semata-mata",
	"sempat", "semua", "semuanya", "semula", "sendiri",
	"sendirian", "sendirinya", "seolah", "seolah-olah", "seorang",
	"sepanjang", "sepantasnya", "sepantasnyalah", "seperlunya",
	"seperti", "sepertinya", "sepihak", "sering", "seringnya",
	"serta", "serupa", "sesaat", "sesama", "sesampai", "sesegera",
	"sesekali", "seseorang", "sesuatu", "sesuatunya", "sesudah",
	"sesudahnya", "setelah", "setempat", "setengah", "seterusnya",
	"setiap", "setiba", "setibanya", "setidak-tidaknya", "setidaknya",
	"setinggi", "seusai", "sewaktu", "siap", "siapa", "siapakah",
	"siapapun", "sini", "sinilah", "soal", "soalnya", "suatu",
	"sudah", "sudahkah", "sudahlah", "supaya", "tadi",
	"tadinya", "tahu", "tahun", "tak", "tambah", "tambahnya",
	"tampak", "tampaknya", "tandas", "tandasnya", "tanpa",
	"tanya", "tanyakan", "tanyanya", "tapi", "tegas", "tegasnya",
	"telah", "tempat", "tengah", "tentang", "tentu", "tentulah",
	"tentunya", "tepat", "terakhir", "terakhirnya", "terasa",
	"terbanyak", "terdahulu", "terdahulunya", "terdapat",
	"terdapatnya", "terendah", "tergolong", "terhadap", "terhadapnya",
	"teringat", "teringat-ingat", "terjadi", "terjadilah",
	"terjadinya", "terkira", "terlalu", "terlebih", "terlihat",
	"termasuk", "ternyata", "tersampaikan", "tersebut", "tersebutlah",
	"tertentu", "tertuju", "terus", "terutama", "tetap", "tetapi",
	"tiap", "tiba", "tiba-tiba", "tidak", "tidakkah", "tidaklah",
	"tiada", "tiadanya", "tiap", "tiap-tiap", "tiga", "tinggi",
	"toh", "turut", "tutur", "tuturnya", "ucap", "ucapan",
	"ucapnya", "ujar", "ujarnya", "umum", "umumnya", "ungkap",
	"ungkapnya", "untuk", "usah", "usai", "waktu", "waktunya",
	"walau", "walaupun", "wong", "yaitu", "yakin", "yakni", "yang",
}

// IndonesianAnalyzer is an analyzer for Indonesian (Bahasa Indonesia) language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.id.IndonesianAnalyzer.
//
// IndonesianAnalyzer uses the StandardTokenizer with Indonesian stop words removal.
type IndonesianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewIndonesianAnalyzer creates a new IndonesianAnalyzer with default Indonesian stop words.
func NewIndonesianAnalyzer() *IndonesianAnalyzer {
	stopSet := GetWordSetFromStrings(IndonesianStopWords, true)
	return NewIndonesianAnalyzerWithWords(stopSet)
}

// NewIndonesianAnalyzerWithWords creates an IndonesianAnalyzer with custom stop words.
func NewIndonesianAnalyzerWithWords(stopWords *CharArraySet) *IndonesianAnalyzer {
	a := &IndonesianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *IndonesianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *IndonesianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *IndonesianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*IndonesianAnalyzer)(nil)
var _ AnalyzerInterface = (*IndonesianAnalyzer)(nil)
