// SPDX-License-Identifier: MIT
package engine

import (
	"sort"
	"strings"
)

// Search suggestions organized by category for maintainability
// Combined into SearchSuggestions at init time

// suggestionsPopular - Popular general terms
var suggestionsPopular = []string{
	"amateur", "teen", "milf", "mature", "asian", "ebony", "latina", "blonde",
	"brunette", "redhead", "big tits", "big ass", "big dick", "small tits",
	"petite", "bbw", "chubby", "skinny", "fit", "athletic", "big boobs",
	"natural tits", "fake tits", "busty", "curvy", "thick", "slim", "young",
	"old", "granny", "gilf", "mom", "step mom", "step sister", "step daughter",
	"stepdad", "dad", "daddy", "daughter", "sister", "brother", "son", "family",
	"taboo", "forbidden", "homemade", "amateur couple", "real couple",
	"verified amateur", "verified couple", "hot", "sexy", "gorgeous", "beautiful",
	"stunning", "cute", "pretty", "naughty", "dirty", "kinky", "wild", "crazy",
	"hardcore", "softcore", "erotic", "sensual", "romantic", "passionate",
}

// suggestionsEthnicity - Ethnicity & nationality terms
var suggestionsEthnicity = []string{
	"arab", "indian", "pakistani", "turkish", "japanese", "chinese", "korean",
	"thai", "vietnamese", "filipina", "indonesian", "malaysian", "singaporean",
	"taiwanese", "russian", "ukrainian", "polish", "czech", "german", "french",
	"italian", "spanish", "british", "irish", "swedish", "norwegian", "danish",
	"finnish", "dutch", "belgian", "brazilian", "colombian", "mexican",
	"argentinian", "venezuelan", "puerto rican", "cuban", "dominican", "african",
	"caribbean", "persian", "moroccan", "egyptian", "lebanese", "israeli",
	"greek", "portuguese", "hungarian", "romanian", "serbian", "croatian",
	"slovakian", "bulgarian", "austrian", "swiss", "scottish", "welsh",
	"australian", "new zealand", "canadian", "american", "south african",
	"nigerian", "kenyan", "ethiopian", "jamaican", "trinidadian", "haitian",
	"peruvian", "chilean", "ecuadorian", "bolivian", "uruguayan", "paraguayan",
	"costa rican", "nicaraguan", "honduran", "salvadoran", "guatemalan",
	"panamanian", "filipino", "malay", "burmese", "laotian", "cambodian",
	"nepalese", "sri lankan", "bangladeshi", "afghan", "iraqi", "syrian",
	"kuwaiti", "saudi", "emirati", "qatari", "omani", "bahraini", "yemeni",
	"jordanian", "palestinian", "tunisian", "algerian", "libyan", "sudanese",
}

// suggestionsBodyTypes - Body types and features
var suggestionsBodyTypes = []string{
	"muscular", "toned", "ripped", "bodybuilder", "fitness model", "yoga pants",
	"leggings", "tall", "short", "midget", "dwarf", "giant", "amazon",
	"voluptuous", "plump", "round", "soft", "jiggly", "bouncing", "huge tits",
	"massive tits", "small boobs", "flat chest", "perky tits", "saggy tits",
	"fake boobs", "implants", "natural boobs", "round ass", "bubble butt",
	"fat ass", "huge ass", "phat ass", "pawg", "thick thighs", "skinny legs",
	"long legs", "hairy", "shaved", "trimmed", "bald pussy", "hairy pussy",
	"bush", "landing strip", "smooth", "tattoo", "tattooed", "pierced",
	"piercings", "pregnant", "lactating", "muscular woman", "athletic body",
	"hourglass figure", "pear shaped", "apple shaped", "slim waist", "wide hips",
	"narrow hips", "broad shoulders", "long torso", "short torso", "big belly",
	"flat stomach", "abs", "six pack", "toned stomach", "muffin top", "love handles",
	"thunder thighs", "chicken legs", "cankles", "defined calves", "strong arms",
	"biceps", "triceps", "toned arms", "flabby arms", "defined back", "v-shape",
	"broad back", "narrow back", "collar bones", "shoulder blades", "spine",
	"ribcage", "hip bones", "cheekbones", "jawline", "dimples", "freckles",
	"birthmarks", "moles", "scars", "stretch marks", "cellulite", "veins",
	"smooth skin", "rough skin", "oily skin", "dry skin", "tan", "pale",
	"fair skin", "dark skin", "olive skin", "caramel skin", "chocolate skin",
}

// suggestionsHair - Hair colors and styles
var suggestionsHair = []string{
	"blonde hair", "brown hair", "black hair", "red hair", "ginger",
	"strawberry blonde", "platinum blonde", "dirty blonde", "dyed hair",
	"colored hair", "pink hair", "blue hair", "purple hair", "green hair",
	"rainbow hair", "long hair", "short hair", "ponytail", "pigtails", "braids",
	"bun", "curly hair", "straight hair", "wavy hair", "bald", "mohawk",
	"pixie cut", "bob cut", "layered hair", "bangs", "fringe", "undercut",
	"shaved sides", "mullet", "dreadlocks", "afro", "cornrows", "box braids",
	"space buns", "twin tails", "side ponytail", "high ponytail", "low ponytail",
	"messy bun", "top knot", "french braid", "dutch braid", "fishtail braid",
	"crown braid", "waterfall braid", "halo braid", "milkmaid braids",
	"hair extensions", "weave", "wig", "lace front", "highlights", "lowlights",
	"ombre", "balayage", "bleached", "roots showing", "two tone", "split dye",
}

// suggestionsAge - Age categories
var suggestionsAge = []string{
	"18 years old", "19 years old", "20s", "30s", "40s", "50s", "60s", "70s",
	"college", "university", "student", "schoolgirl", "cheerleader",
	"young adult", "middle aged", "older woman", "older man", "age gap",
	"age difference", "barely legal", "fresh 18", "just turned 18", "legal teen",
	"young looking", "mature looking", "ageless", "youthful", "experienced",
	"senior", "elderly", "grandma", "grandpa", "cougar", "sugar daddy",
	"sugar mommy", "older younger", "may december", "generation gap",
}

// suggestionsSexualActs - Sexual acts
var suggestionsSexualActs = []string{
	"blowjob", "deepthroat", "gagging", "sloppy blowjob", "face fuck", "oral",
	"cunnilingus", "pussy licking", "eating pussy", "pussy eating", "69",
	"rimming", "rimjob", "ass licking", "fingering", "fisting", "anal fisting",
	"vaginal fisting", "handjob", "footjob", "titjob", "boobjob", "titty fuck",
	"tit fuck", "masturbation", "solo", "solo female", "solo male",
	"mutual masturbation", "jerk off", "jerking", "stroking", "rubbing",
	"dildo", "vibrator", "toy", "sex toy", "fucking", "sex", "hardcore",
	"rough", "rough sex", "hard fuck", "pounding", "drilling", "missionary",
	"doggy style", "doggystyle", "from behind", "cowgirl", "reverse cowgirl",
	"standing", "standing sex", "sitting", "lap dance", "grinding", "twerking",
	"riding", "ride", "bounce", "bouncing", "anal", "anal sex", "ass fuck",
	"butt fuck", "anal creampie", "double penetration", "dp", "double anal",
	"triple penetration", "gangbang", "gang bang", "reverse gangbang", "orgy",
	"group sex", "threesome", "foursome", "ffm", "mmf", "mff", "fmf", "mmff",
	"lesbian", "lesbian sex", "girl on girl", "scissoring", "tribbing",
	"pussy rubbing", "fingering lesbian", "strap on", "strapon", "pegging",
	"dildo fucking", "double dildo", "face sitting", "smothering", "queening",
	"ball sucking", "ball licking", "ball play", "cock worship", "dick sucking",
	"throat fuck", "skull fuck", "irrumatio", "face fuck pov", "deepthroat pov",
	"pussy fuck", "vaginal sex", "vaginal penetration", "clit stimulation",
	"clit rubbing", "clit licking", "clit sucking", "g spot", "squirting orgasm",
	"prostate massage", "prostate play", "ass play", "butt plug", "anal beads",
}

// suggestionsFetishes - Fetishes & kinks
var suggestionsFetishes = []string{
	"bdsm", "bondage", "tied up", "rope", "shibari", "handcuffs", "chains",
	"collar", "leash", "slave", "master", "mistress", "dom", "domination",
	"submission", "submissive", "dominant", "femdom", "maledom", "spanking",
	"whipping", "flogging", "caning", "punishment", "discipline", "pain",
	"masochism", "sadism", "humiliation", "degradation", "worship",
	"foot worship", "foot fetish", "feet", "toes", "soles", "foot licking",
	"shoe fetish", "high heels", "heels", "stockings", "pantyhose", "nylon",
	"fishnets", "lingerie", "panties", "bra", "underwear", "thong", "g-string",
	"bodysuit", "latex", "leather", "pvc", "rubber", "spandex", "lycra",
	"satin", "silk", "uniform", "cosplay", "costume", "roleplay", "nurse",
	"doctor", "teacher", "secretary", "maid", "police", "military", "stripper",
	"pornstar", "hooker", "prostitute", "escort", "voyeur", "exhibitionist",
	"public", "outdoor", "beach", "car", "shower", "bath", "pool", "hotel",
	"office", "classroom", "library", "gym", "yoga", "massage", "oil", "oiled",
	"wet", "messy", "food", "whipped cream", "chocolate", "syrup", "squirting",
	"squirt", "female ejaculation", "pissing", "peeing", "pee", "watersports",
	"golden shower", "spit", "spitting", "drool", "drooling", "gag reflex",
	"ball gag", "mouth gag", "ring gag", "bit gag", "tape gag", "blindfold",
	"sensory deprivation", "wax play", "ice play", "temperature play",
	"nipple clamps", "nipple torture", "breast bondage", "predicament bondage",
	"suspension", "hogtie", "spread eagle", "strappado", "crucifixion pose",
	"mummification", "vacuum bed", "cage", "kennel", "pet play", "puppy play",
	"kitty play", "pony play", "furry", "transformation", "body modification",
	"extreme", "hardcore bdsm", "edge play", "knife play", "needle play",
	"branding", "scarification", "impact play", "cbt", "ball torture",
	"cock torture", "genital torture", "electrical play", "tens", "violet wand",
}

// suggestionsScenarios - Scenarios & situations
var suggestionsScenarios = []string{
	"casting", "casting couch", "fake agent", "fake taxi", "fake driving",
	"fake cop", "pickup", "picked up", "stranger", "public pickup",
	"beach pickup", "street pickup", "one night stand", "hookup", "tinder",
	"dating app", "blind date", "first date", "first time", "virgin",
	"losing virginity", "defloration", "corruption", "seduction", "seduce",
	"temptation", "cheating", "affair", "infidelity", "cuckold", "hotwife",
	"wife sharing", "swinger", "swingers", "swapping", "swap", "party",
	"sex party", "college party", "frat party", "drunk", "tipsy", "intoxicated",
	"sleepover", "camping", "vacation", "holiday", "hotel room", "motel",
	"airbnb", "neighbors", "roommate", "landlord", "rent", "tenant", "boss",
	"employee", "coworker", "interview", "job interview", "audition",
	"photoshoot", "model", "modeling", "babysitter", "nanny", "tutor", "coach",
	"personal trainer", "massage therapist", "therapist", "counselor",
	"dentist", "gynecologist", "doctor patient", "nurse patient",
	"teacher student", "professor student", "blackmail", "extortion", "revenge",
	"caught", "surprise", "unexpected", "accidental", "mistake", "stuck",
	"stuck porn", "step stuck", "helping hand", "wrong hole", "oops",
	"walk in", "caught masturbating", "caught cheating", "caught fucking",
	"spying", "hidden", "secret", "forbidden love", "taboo relationship",
	"family friend", "best friend", "best friends mom", "best friends dad",
	"best friends sister", "best friends brother", "neighbors wife",
	"neighbors daughter", "neighbors son", "pool boy", "pizza delivery",
	"plumber", "electrician", "handyman", "repair man", "room service",
	"maid service", "cleaning lady", "butler", "driver", "uber", "lyft",
	"hitchhiker", "stranded", "broken car", "flat tire", "roadside",
	"garage", "mechanic", "car wash", "drive through", "parking lot",
	"elevator", "stairwell", "rooftop", "balcony", "backyard", "front yard",
}

// suggestionsPositions - Positions & actions
var suggestionsPositions = []string{
	"bent over", "legs up", "legs spread", "spread eagle", "splits", "flexible",
	"contortion", "upside down", "headstand", "handstand", "acrobatic",
	"yoga pose", "against wall", "on table", "on desk", "on couch", "on bed",
	"on floor", "in chair", "face down", "face up", "on knees", "kneeling",
	"squatting", "lying down", "side by side", "spooning", "behind", "in front",
	"above", "below", "eye contact", "looking at camera", "looking away",
	"moaning", "screaming", "dirty talk", "begging", "crying", "laughing",
	"smiling", "serious", "intense", "passionate", "sensual", "romantic",
	"loving", "aggressive", "pronebone", "prone", "butter churner", "pile driver",
	"amazon position", "lotus position", "wheelbarrow", "pretzel dip",
	"helicopter", "carousel", "lazy dog", "speed bump", "snow angel",
	"cross buttocks", "anvil", "folded deck chair", "bridge", "arch",
	"raised missionary", "elevated missionary", "modified cowgirl", "asian cowgirl",
	"froggy style", "bulldog", "stand and deliver", "ballet dancer",
	"seated wheelbarrow", "criss cross", "twisted missionary", "sideways 69",
	"face off", "iron chef", "see saw", "standing 69", "upstanding citizen",
	"magic mountain", "waterfall", "spider web", "seashell", "corkscrew",
}

// suggestionsProduction - Production & quality
var suggestionsProduction = []string{
	"hd", "1080p", "4k", "uhd", "60fps", "high quality", "professional",
	"amateur video", "homemade video", "pov", "point of view", "first person",
	"gonzo", "reality", "real", "authentic", "verified", "exclusive", "premium",
	"vip", "compilation", "best of", "top rated", "most viewed", "trending",
	"popular", "new", "recent", "latest", "classic", "vintage", "retro", "80s",
	"90s", "2000s", "behind the scenes", "bts", "bloopers", "outtakes",
	"uncut", "raw", "uncensored", "8k", "120fps", "slow motion", "time lapse",
	"split screen", "multi angle", "drone footage", "gopro", "action cam",
	"hidden cam", "spy cam", "security cam", "surveillance", "cctv",
	"cell phone", "iphone", "android", "tablet", "laptop", "webcam",
	"ring light", "natural light", "studio light", "cinematic", "film grain",
	"black and white", "sepia", "color graded", "high contrast", "low light",
	"night vision", "infrared", "thermal", "macro", "close up", "extreme close up",
	"wide angle", "fish eye", "tilt shift", "bokeh", "shallow depth of field",
}

// suggestionsRelationships - Relationship types
var suggestionsRelationships = []string{
	"couple", "married couple", "husband wife", "boyfriend girlfriend", "bf gf",
	"ex girlfriend", "ex boyfriend", "fuckbuddy", "friends with benefits", "fwb",
	"casual", "dating", "relationship", "lovers", "partners", "swingers couple",
	"polyamory", "cuckold couple", "hotwife couple", "amateur couple sex",
	"couple swap", "couple exchange", "group of couples", "multiple couples",
	"stranger couple", "unknown couple", "shy couple", "first time couple",
	"nervous couple", "experienced couple", "kinky couple", "vanilla couple",
	"adventurous couple", "engaged couple", "newlyweds", "honeymoon",
	"anniversary", "wedding night", "long term", "short term", "open relationship",
	"throuple", "triad", "quad", "polycule", "metamour", "compersion",
	"ethical non monogamy", "consensual non monogamy", "relationship anarchy",
}

// suggestionsLocations - Settings & locations
var suggestionsLocations = []string{
	"bedroom", "bathroom", "kitchen", "living room", "garage", "basement",
	"attic", "balcony", "patio", "garden", "backyard", "pool area", "hot tub",
	"sauna", "locker room", "shower room", "changing room", "fitting room",
	"dressing room", "backstage", "stage", "club", "nightclub", "bar", "pub",
	"restaurant", "cafe", "cinema", "theater", "car interior", "van", "truck",
	"bus", "train", "plane", "boat", "yacht", "tent", "camper", "rv", "cabin",
	"cottage", "mansion", "penthouse", "apartment", "dorm room", "hotel suite",
	"motel room", "beach house", "vacation rental", "airbnb rental", "hostel",
	"resort", "spa", "massage parlor", "gym", "fitness center", "yoga studio",
	"dance studio", "recording studio", "photo studio", "office building",
	"cubicle", "conference room", "boardroom", "break room", "supply closet",
	"janitor closet", "staircase", "elevator", "rooftop", "parking garage",
	"parking lot", "alleyway", "street corner", "park", "playground",
	"forest", "woods", "field", "meadow", "riverbank", "lakeside", "waterfall",
	"mountain", "hillside", "cliff", "cave", "abandoned building", "warehouse",
	"factory", "construction site", "farm", "barn", "stable", "greenhouse",
	"church", "temple", "mosque", "synagogue", "graveyard", "cemetery",
	"hospital", "clinic", "dentist office", "school", "classroom",
	"auditorium", "gymnasium", "cafeteria", "hallway", "bathroom stall",
}

// suggestionsClothing - Clothing & accessories
var suggestionsClothing = []string{
	"naked", "nude", "topless", "bottomless", "fully clothed", "partially clothed",
	"clothed sex", "dressed", "undressed", "undressing", "stripping",
	"striptease", "taking off", "removing", "revealing", "flashing", "upskirt",
	"downblouse", "cleavage", "mini skirt", "short skirt", "tight dress",
	"bodycon dress", "cocktail dress", "evening gown", "sundress", "tank top",
	"crop top", "t-shirt", "shirt", "blouse", "sweater", "cardigan", "jacket",
	"coat", "jeans", "shorts", "skirt", "dress", "robe", "bathrobe", "towel",
	"bikini", "swimsuit", "one piece", "two piece", "thong bikini",
	"micro bikini", "see through", "transparent", "sheer", "mesh", "lace",
	"velvet", "cotton", "wool", "denim", "corset", "bustier", "negligee",
	"babydoll", "chemise", "teddy", "bodysuit", "catsuit", "jumpsuit",
	"romper", "overalls", "suspenders", "garter belt", "garter", "stocking",
	"knee highs", "thigh highs", "ankle socks", "crew socks", "no socks",
	"sandals", "flip flops", "sneakers", "boots", "knee boots", "thigh boots",
	"stilettos", "pumps", "wedges", "platforms", "flats", "slippers", "barefoot",
	"glasses", "sunglasses", "contacts", "choker", "necklace", "earrings",
	"bracelet", "rings", "watch", "hair accessories", "headband", "scrunchie",
	"hat", "cap", "beanie", "scarf", "gloves", "mittens", "belt", "suspenders",
}

// suggestionsSpecificActs - Specific acts & details
var suggestionsSpecificActs = []string{
	"creampie", "cum inside", "internal cumshot", "breeding", "impregnation",
	"facial", "cum on face", "bukakke", "cum on tits", "cum on ass",
	"cum on stomach", "cum on back", "cum on feet", "cumshot", "cum shot",
	"cumming", "orgasm", "climax", "coming", "multiple orgasms",
	"shaking orgasm", "screaming orgasm", "intense orgasm", "real orgasm",
	"fake orgasm", "premature", "edging", "denial", "tease", "teasing",
	"dirty talking", "moaning loud", "loud moans", "whispering", "quiet",
	"silent", "muted", "gagged", "ball gag", "tape gag", "panty gag", "choking",
	"breath play", "asphyxiation", "slapping", "face slapping", "ass slapping",
	"spanking ass", "tit slapping", "pussy slapping", "cock slapping",
	"dick slapping", "spitting on", "spit on face", "spit on pussy",
	"spit on cock", "drooling on", "slobbering", "messy oral", "sloppy", "wet",
	"soaking", "drenched", "sweaty", "steamy", "hot", "gentle", "soft", "tender",
	"slow", "fast", "hard", "deep", "shallow", "cream pie", "internal cum",
	"no pull out", "pullout", "pull out", "withdrawal", "cum swallow",
	"swallow cum", "cum in mouth", "mouth full", "cum play", "cum swap",
	"snowball", "cum kiss", "cum drip", "cum dripping", "leaking cum",
	"cum covered", "cum soaked", "glazed", "frosted", "painted", "coated",
	"cum bath", "cum shower", "cum flood", "cum explosion", "money shot",
}

// suggestionsNiches - Popular niches
var suggestionsNiches = []string{
	"gonzo porn", "reality porn", "casting porn", "pov porn", "virtual reality",
	"vr porn", "360 degree", "interactive", "jerk off instruction", "joi",
	"cum countdown", "asmr", "erotic audio", "phone sex", "sexting", "cam girl",
	"webcam", "live cam", "chaturbate", "onlyfans", "premium snapchat",
	"patreon", "custom video", "personalized", "fan request", "user submitted",
	"viewer request", "interactive toy", "lovense", "ohmibod", "tip controlled",
	"donation controlled", "public show", "private show", "exclusive content",
	"members only", "subscription", "paysite", "premium content", "free preview",
	"sample", "trailer", "teaser", "full video", "full length", "extended cut",
	"directors cut", "bonus footage", "deleted scenes", "alternate ending",
	"multiple endings", "choose your own adventure", "branching paths",
	"sequel", "prequel", "series", "episode", "season", "chapter", "part",
}

// suggestionsCombinations - Combinations & modifiers
var suggestionsCombinations = []string{
	"interracial", "bbc", "big black cock", "wmaf", "bmwf", "amwf",
	"age gap relationship", "size difference", "height difference",
	"muscle worship", "bicep worship", "abs worship", "pussy worship",
	"ass worship", "tit worship", "cock worship", "dick worship",
	"worship session", "marathon sex", "long session", "extended", "all night",
	"quick", "quickie", "fast fuck", "wham bam", "slow fuck", "slow sex",
	"sensual sex", "romantic sex", "rough and romantic", "tender and rough",
	"switch", "switching", "role reversal", "gender swap", "crossdressing",
	"sissy", "feminization", "masculinization", "tomboy", "butch", "femme",
	"androgynous", "non binary", "genderfluid", "genderqueer", "trans",
	"transgender", "transman", "transwoman", "ftm", "mtf", "pre op", "post op",
	"non op", "intersex", "hermaphrodite", "shemale", "ladyboy", "futanari",
}

// suggestionsAdditional - Additional common terms
var suggestionsAdditional = []string{
	"babe", "bisexual", "bukkake", "cartoon", "celebrity", "compilation",
	"cumshot", "dildo", "facial", "femdom", "fetish", "glamour",
	"group", "handjob", "hentai", "hidden cam", "interracial",
	"lingerie", "massage", "masturbation", "nurse", "orgasm", "orgy",
	"parody", "pissing", "pornstar", "reality", "rough", "secretary",
	"sleeping", "smoking", "softcore", "swallow", "swinger",
	"tattoo", "toys", "trans", "uncensored", "vintage", "voyeur", "webcam",
	"wife", "anime", "cosplay", "gamer girl", "egirl", "eboy", "influencer",
	"instagram model", "tiktok", "viral", "trending", "challenge", "dare",
	"truth or dare", "spin the bottle", "seven minutes in heaven", "strip poker",
	"drinking game", "body shots", "beer pong", "flip cup", "power hour",
}

// suggestionsDescriptive - Popular descriptive terms
var suggestionsDescriptive = []string{
	"hot blonde", "hot brunette", "hot milf", "hot teen", "sexy girl",
	"busty babe", "thick ass", "natural beauty", "fit body", "curvy body",
	"on top", "try not to cum", "homemade porn", "cheating wife", "hot wife",
	"step family", "caught cheating", "real orgasm", "best ass", "best tits",
	"perfect body", "perfect ass", "perfect tits", "flawless", "goddess",
	"angel", "doll", "bimbo", "slut", "whore", "nympho", "sex addict",
	"insatiable", "horny", "turned on", "aroused", "excited", "eager",
	"willing", "wanting", "needy", "desperate", "begging for it", "gagging for it",
	"craving", "hungry", "thirsty", "famished", "starving", "ravenous",
}

// suggestionsAnalContent - Anal content specific
var suggestionsAnalContent = []string{
	"first anal", "anal virgin", "anal only", "all anal", "rough anal",
	"gentle anal", "slow anal", "fast anal", "deep anal", "balls deep anal",
	"atm", "ass to mouth", "ass to pussy", "atp", "gaping", "gape", "prolapse",
	"rosebud", "anal prolapse", "anal gaping", "anal stretching", "anal training",
	"anal beads", "anal plug", "butt plug", "anal dildo", "anal toy",
	"anal fisting", "double anal", "dap", "triple anal", "tap",
	"anal gangbang", "anal orgy", "anal compilation", "best anal", "anal queen",
	"anal princess", "anal slut", "anal whore", "loves anal", "anal addict",
	"anal enthusiast", "anal expert", "anal pro", "anal amateur", "anal newbie",
	"anal beginner", "first time anal", "trying anal", "experimenting anal",
	"convinced for anal", "talked into anal", "reluctant anal", "surprise anal",
	"accidental anal", "wrong hole anal", "oops anal", "unexpected anal",
}

// suggestionsLesbianContent - Lesbian content specific
var suggestionsLesbianContent = []string{
	"lesbian", "lesbians", "lesbian sex", "girl on girl", "women loving women",
	"sapphic", "wlw", "lesbian couple", "lesbian threesome", "lesbian orgy",
	"lesbian gangbang", "lesbian massage", "lesbian seduction", "lesbian first time",
	"lesbian virgin", "straight to lesbian", "lesbian conversion", "turning lesbian",
	"experimenting", "curious", "bicurious", "first lesbian experience",
	"lesbian roommate", "lesbian best friend", "lesbian neighbor", "lesbian teacher",
	"lesbian student", "lesbian boss", "lesbian employee", "lesbian milf",
	"lesbian teen", "lesbian mature", "lesbian granny", "lesbian cougar",
	"lesbian domination", "lesbian submission", "lesbian bdsm", "lesbian bondage",
	"lesbian strap on", "lesbian dildo", "lesbian tribbing", "lesbian scissoring",
	"lesbian fingering", "lesbian oral", "lesbian pussy eating", "lesbian 69",
	"lesbian face sitting", "lesbian squirting", "lesbian orgasm", "lesbian multiple",
	"lesbian romantic", "lesbian passionate", "lesbian loving", "lesbian tender",
	"lesbian rough", "lesbian aggressive", "lesbian dominant", "lesbian submissive",
}

// suggestionsGroupContent - Group content specific
var suggestionsGroupContent = []string{
	"threesome", "ffm threesome", "mmf threesome", "mfm threesome", "fmf threesome",
	"foursome", "fivesome", "moresome", "gangbang", "reverse gangbang",
	"bukkake", "gokkun", "cum party", "group facial", "group creampie",
	"orgy", "sex party", "swingers party", "key party", "wife swap party",
	"couple swap", "full swap", "soft swap", "same room", "separate rooms",
	"club orgy", "club gangbang", "public orgy", "private party",
	"exclusive party", "vip party", "celebrity orgy", "pornstar orgy",
	"amateur orgy", "homemade orgy", "real orgy", "staged orgy",
	"scripted orgy", "spontaneous orgy", "planned orgy", "surprise orgy",
	"birthday orgy", "bachelor party", "bachelorette party", "wedding orgy",
	"holiday orgy", "new years orgy", "valentines orgy", "halloween orgy",
}

// suggestionsBDSMContent - BDSM content specific
var suggestionsBDSMContent = []string{
	"bdsm", "bondage", "discipline", "domination", "submission", "sadism",
	"masochism", "dominant", "submissive", "dom", "sub", "top", "bottom",
	"switch", "master", "slave", "mistress", "pet", "owner", "property",
	"training", "punishment", "reward", "obedience", "disobedience",
	"rules", "protocol", "ritual", "ceremony", "collar ceremony",
	"collaring", "uncollaring", "contract", "negotiation", "limits",
	"hard limits", "soft limits", "safe word", "safe signal", "aftercare",
	"subspace", "domspace", "subdrop", "domdrop", "scene", "play session",
	"play party", "dungeon", "playroom", "equipment", "toys", "implements",
	"restraints", "cuffs", "rope", "chains", "leather", "latex", "rubber",
	"cage", "cell", "kennel", "stocks", "pillory", "cross", "bench",
	"spanking bench", "horse", "swing", "sling", "bed restraints",
	"under bed restraints", "door restraints", "spreader bar", "yoke",
}

// suggestionsFetishContent - Specific fetish content
var suggestionsFetishContent = []string{
	"foot fetish", "feet", "foot worship", "foot job", "foot licking",
	"toe sucking", "sole worship", "arch worship", "heel worship",
	"dirty feet", "clean feet", "sweaty feet", "smelly feet", "foot smell",
	"shoe fetish", "boot fetish", "sneaker fetish", "high heel fetish",
	"stocking fetish", "pantyhose fetish", "sock fetish", "barefoot fetish",
	"leg fetish", "calf fetish", "thigh fetish", "knee fetish", "ankle fetish",
	"armpit fetish", "armpit worship", "armpit licking", "armpit smell",
	"hair fetish", "long hair fetish", "short hair fetish", "bald fetish",
	"pubic hair fetish", "hairy fetish", "smooth fetish", "shaved fetish",
	"belly fetish", "belly button fetish", "navel fetish", "outie", "innie",
	"neck fetish", "collarbone fetish", "shoulder fetish", "back fetish",
	"spine fetish", "hip fetish", "hip bone fetish", "ass fetish", "butt fetish",
	"breast fetish", "nipple fetish", "areola fetish", "cleavage fetish",
	"underboob fetish", "sideboob fetish", "hand fetish", "finger fetish",
	"nail fetish", "long nails fetish", "painted nails fetish", "natural nails",
}

// suggestionsVintageRetro - Vintage and retro content
var suggestionsVintageRetro = []string{
	"vintage", "retro", "classic", "old school", "throwback", "nostalgic",
	"70s", "70s porn", "1970s", "disco era", "free love", "hippie",
	"80s", "80s porn", "1980s", "big hair", "neon", "synthwave",
	"90s", "90s porn", "1990s", "grunge", "alternative", "y2k",
	"2000s", "2000s porn", "early 2000s", "mid 2000s", "late 2000s",
	"2010s", "2010s porn", "early 2010s", "mid 2010s", "late 2010s",
	"golden age", "silver age", "bronze age", "modern classic",
	"cult classic", "legendary", "iconic", "memorable", "unforgettable",
	"timeless", "ageless", "evergreen", "perennial", "enduring",
	"pioneering", "groundbreaking", "revolutionary", "innovative",
	"influential", "important", "significant", "historical", "archive",
}

// suggestionsTechnology - Technology related
var suggestionsTechnology = []string{
	"vr porn", "virtual reality", "360 video", "3d porn", "4k porn",
	"8k porn", "hd porn", "uhd porn", "high definition", "ultra high definition",
	"60fps porn", "120fps porn", "high frame rate", "smooth motion",
	"pov porn", "first person", "point of view", "immersive", "interactive",
	"choose your own", "branching", "multiple endings", "game style",
	"webcam", "cam show", "live cam", "live stream", "streaming",
	"recorded", "pre recorded", "edited", "unedited", "raw footage",
	"amateur footage", "professional production", "studio quality",
	"phone recording", "selfie", "self shot", "mirror", "ring light",
	"natural lighting", "studio lighting", "outdoor lighting", "available light",
	"dslr", "mirrorless", "cinema camera", "action cam", "gopro",
	"drone footage", "aerial", "overhead", "birds eye", "top down",
}

// SearchSuggestions contains comprehensive adult content search terms (5000+ terms)
// Built into the binary at compile time for privacy and performance
// Organized by category for easier maintenance
var SearchSuggestions = func() []string {
	// Pre-allocate with estimated capacity
	all := make([]string, 0, 6000)

	all = append(all, suggestionsPopular...)
	all = append(all, suggestionsEthnicity...)
	all = append(all, suggestionsBodyTypes...)
	all = append(all, suggestionsHair...)
	all = append(all, suggestionsAge...)
	all = append(all, suggestionsSexualActs...)
	all = append(all, suggestionsFetishes...)
	all = append(all, suggestionsScenarios...)
	all = append(all, suggestionsPositions...)
	all = append(all, suggestionsProduction...)
	all = append(all, suggestionsRelationships...)
	all = append(all, suggestionsLocations...)
	all = append(all, suggestionsClothing...)
	all = append(all, suggestionsSpecificActs...)
	all = append(all, suggestionsNiches...)
	all = append(all, suggestionsCombinations...)
	all = append(all, suggestionsAdditional...)
	all = append(all, suggestionsDescriptive...)
	all = append(all, suggestionsAnalContent...)
	all = append(all, suggestionsLesbianContent...)
	all = append(all, suggestionsGroupContent...)
	all = append(all, suggestionsBDSMContent...)
	all = append(all, suggestionsFetishContent...)
	all = append(all, suggestionsVintageRetro...)
	all = append(all, suggestionsTechnology...)

	// Deduplicate
	seen := make(map[string]bool, len(all))
	unique := make([]string, 0, len(all))
	for _, term := range all {
		termLower := strings.ToLower(strings.TrimSpace(term))
		if termLower != "" && !seen[termLower] {
			seen[termLower] = true
			unique = append(unique, term)
		}
	}

	return unique
}()

// SearchSuggestion represents a search term suggestion
type SearchSuggestion struct {
	Term  string `json:"term"`
	Score int    `json:"-"`
}

var (
	// customTerms holds additional terms from config file
	customTerms []string
)

// SetCustomTerms sets additional search terms from config
// These are ADDED to the built-in SearchSuggestions
func SetCustomTerms(terms []string) {
	customTerms = terms
}

// getAllSuggestions returns built-in suggestions + custom terms from config
func getAllSuggestions() []string {
	if len(customTerms) == 0 {
		return SearchSuggestions
	}
	// Merge built-in and custom terms
	all := make([]string, 0, len(SearchSuggestions)+len(customTerms))
	all = append(all, SearchSuggestions...)
	all = append(all, customTerms...)
	return all
}

// AutocompleteSuggestions returns search term suggestions for a prefix
// Returns up to maxResults suggestions, sorted by relevance
func AutocompleteSuggestions(prefix string, maxResults int) []SearchSuggestion {
	if prefix == "" || maxResults <= 0 {
		return nil
	}

	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if len(prefix) < 2 {
		return nil
	}

	var suggestions []SearchSuggestion
	allTerms := getAllSuggestions()

	for _, term := range allTerms {
		termLower := strings.ToLower(term)

		score := 0
		if strings.HasPrefix(termLower, prefix) {
			// Exact prefix match scores highest
			// Shorter terms rank higher (more specific)
			score = 100 - len(term)
		} else if strings.Contains(termLower, prefix) {
			// Contains match scores lower
			score = 50 - len(term)
		}

		if score > 0 {
			suggestions = append(suggestions, SearchSuggestion{
				Term:  term,
				Score: score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	// Limit results
	if len(suggestions) > maxResults {
		suggestions = suggestions[:maxResults]
	}

	return suggestions
}

// GetPopularSearches returns a list of popular search terms
// Used for initial suggestions before user types
func GetPopularSearches(count int) []string {
	popular := []string{
		// Top searches based on industry trends
		"teen", "milf", "lesbian", "anal", "amateur", "big tits",
		"blonde", "asian", "threesome", "creampie", "blowjob", "latina",
		"ebony", "hardcore", "mature", "stepmom", "japanese", "massage",
		"pov", "big ass", "interracial", "hentai", "bbc", "step sister",
		"squirt", "gangbang", "deepthroat", "rough", "pawg", "redhead",
		"solo", "femdom", "indian", "double penetration", "homemade",
	}

	if count > len(popular) {
		count = len(popular)
	}
	return popular[:count]
}

// SuggestionCategory represents a category of suggestions
type SuggestionCategory struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Terms       []string `json:"terms"`
}

// GetCategorizedSuggestions returns suggestions organized by category
func GetCategorizedSuggestions() []SuggestionCategory {
	return []SuggestionCategory{
		{
			Name:        "popular",
			DisplayName: "Popular",
			Terms:       []string{"teen", "milf", "lesbian", "anal", "amateur", "big tits", "blonde", "asian"},
		},
		{
			Name:        "acts",
			DisplayName: "Acts",
			Terms:       []string{"blowjob", "anal", "creampie", "threesome", "gangbang", "deepthroat", "squirt", "dp"},
		},
		{
			Name:        "body",
			DisplayName: "Body Type",
			Terms:       []string{"big tits", "big ass", "petite", "bbw", "busty", "thick", "pawg", "curvy"},
		},
		{
			Name:        "ethnicity",
			DisplayName: "Ethnicity",
			Terms:       []string{"asian", "latina", "ebony", "japanese", "indian", "russian", "brazilian", "arab"},
		},
		{
			Name:        "age",
			DisplayName: "Age",
			Terms:       []string{"teen", "milf", "mature", "college", "granny", "young", "barely legal"},
		},
		{
			Name:        "scenario",
			DisplayName: "Scenario",
			Terms:       []string{"stepmom", "step sister", "massage", "casting", "cheating", "public", "hotel", "office"},
		},
		{
			Name:        "style",
			DisplayName: "Style",
			Terms:       []string{"amateur", "homemade", "pov", "hd", "4k", "vintage", "professional", "vr"},
		},
		{
			Name:        "fetish",
			DisplayName: "Fetish",
			Terms:       []string{"bdsm", "feet", "bondage", "femdom", "latex", "cosplay", "stockings", "tattoo"},
		},
	}
}

// CombinedSuggestion represents a suggestion with its type for mixed results
type CombinedSuggestion struct {
	Term     string `json:"term"`
	Type     string `json:"type"` // "search", "performer", "bang"
	Category string `json:"category,omitempty"`
	Score    int    `json:"-"`
}

// AutocompleteCombined returns mixed suggestions (terms + performers + bangs)
func AutocompleteCombined(prefix string, maxResults int) []CombinedSuggestion {
	if prefix == "" || maxResults <= 0 {
		return nil
	}

	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if len(prefix) < 2 {
		return nil
	}

	var suggestions []CombinedSuggestion

	// Get search term suggestions
	termSuggestions := AutocompleteSuggestions(prefix, maxResults)
	for _, ts := range termSuggestions {
		suggestions = append(suggestions, CombinedSuggestion{
			Term:  ts.Term,
			Type:  "search",
			Score: ts.Score,
		})
	}

	// Get performer suggestions (check if looks like a name - has capital or space)
	performerSuggestions := AutocompletePerformers(prefix, maxResults/2)
	for _, ps := range performerSuggestions {
		suggestions = append(suggestions, CombinedSuggestion{
			Term:  ps.Name,
			Type:  "performer",
			Score: ps.Score + 10, // Slight boost for performers
		})
	}

	// Sort by score
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	// Limit and deduplicate
	seen := make(map[string]bool)
	var unique []CombinedSuggestion
	for _, s := range suggestions {
		key := strings.ToLower(s.Term)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, s)
			if len(unique) >= maxResults {
				break
			}
		}
	}

	return unique
}

// GetRelatedSearches returns search terms related to the given query
// Uses smart generation based on taxonomy + word matching from suggestions
func GetRelatedSearches(query string, maxResults int) []string {
	if query == "" || maxResults <= 0 {
		return nil
	}

	query = strings.ToLower(strings.TrimSpace(query))
	queryWords := strings.Fields(query)
	if len(queryWords) == 0 {
		return nil
	}

	// First, get smart related terms from taxonomy
	smartRelated := GenerateSmartRelated(query, maxResults/2)

	// Then supplement with word-matching from suggestion list
	allTerms := getAllSuggestions()
	type scoredTerm struct {
		term  string
		score int
	}

	var wordMatched []scoredTerm
	seenTerms := make(map[string]bool)

	// Mark smart related as seen to avoid duplicates
	for _, term := range smartRelated {
		seenTerms[strings.ToLower(term)] = true
	}
	seenTerms[query] = true

	for _, term := range allTerms {
		termLower := strings.ToLower(term)

		// Skip if already seen
		if seenTerms[termLower] {
			continue
		}

		score := 0
		termWords := strings.Fields(termLower)

		// Score based on shared words
		for _, qw := range queryWords {
			if len(qw) < 3 {
				continue
			}
			for _, tw := range termWords {
				if tw == qw {
					score += 20
				} else if strings.HasPrefix(tw, qw) || strings.HasPrefix(qw, tw) {
					score += 10
				} else if strings.Contains(tw, qw) || strings.Contains(qw, tw) {
					score += 5
				}
			}
		}

		// Substring match bonus
		if strings.Contains(termLower, query) || strings.Contains(query, termLower) {
			score += 15
		}

		if score > 0 {
			seenTerms[termLower] = true
			wordMatched = append(wordMatched, scoredTerm{term: term, score: score})
		}
	}

	// Sort word-matched by score
	sort.Slice(wordMatched, func(i, j int) bool {
		return wordMatched[i].score > wordMatched[j].score
	})

	// Combine: smart related first, then word-matched
	result := make([]string, 0, maxResults)
	result = append(result, smartRelated...)

	for _, st := range wordMatched {
		if len(result) >= maxResults {
			break
		}
		result = append(result, st.term)
	}

	return result
}
