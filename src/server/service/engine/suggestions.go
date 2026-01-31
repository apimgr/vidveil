// SPDX-License-Identifier: MIT
package engine

import (
	"sort"
	"strings"
)

// Search suggestions organized by category for maintainability
// Combined into SearchSuggestions at init time
// Total: 5000+ unique terms across 37 categories

// suggestionsPopular - Popular general terms (~100 terms)
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
	// Additional popular terms
	"pornstar", "celebrity look alike", "model", "instagram", "tiktok star",
	"influencer", "cam girl", "onlyfans", "fansly", "manyvids", "clips4sale",
	"premium", "exclusive", "viral", "trending now", "most watched", "top rated",
	"editor choice", "fan favorite", "staff pick", "recommended", "suggested",
	"new release", "just added", "fresh content", "daily update", "weekly best",
	"monthly top", "all time best", "legendary", "iconic", "classic scene",
	"must watch", "cant miss", "essential", "highly rated", "five star",
}

// suggestionsEthnicity - Ethnicity & nationality terms (~150 terms)
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
	// Additional ethnicity terms
	"mongolian", "kazakh", "uzbek", "turkmen", "kyrgyz", "tajik", "armenian",
	"azerbaijani", "georgian", "belarusian", "moldovan", "latvian", "lithuanian",
	"estonian", "albanian", "macedonian", "montenegrin", "bosnian", "slovenian",
	"luxembourgish", "icelandic", "maltese", "cypriot", "tibetan", "uyghur",
	"hong kong", "macanese", "bruneian", "timorese", "papuan", "fijian",
	"samoan", "tongan", "hawaiian", "maori", "aboriginal", "native american",
	"inuit", "alaskan native", "pacific islander", "polynesian", "melanesian",
	"micronesian", "creole", "mestizo", "mulatto", "mixed race", "biracial",
	"multiracial", "eurasian", "afro latina", "afro asian", "wasian",
}

// suggestionsBodyTypes - Body types and features (~150 terms)
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
	// Additional body type terms
	"thicc", "extra thicc", "slim thick", "thiccc", "mega curves", "all natural",
	"enhanced", "augmented", "breast reduction", "breast lift", "perky butt",
	"firm butt", "soft butt", "hard body", "soft body", "supple", "pliant",
	"lithe", "svelte", "willowy", "statuesque", "diminutive", "compact",
	"rubenesque", "full figured", "plus size", "ssbbw", "feedee", "feeder",
	"skinny fat", "dadbod", "beer belly", "pot belly", "barrel chest",
	"swimmer build", "runner build", "dancer body", "gymnast body", "yoga body",
	"crossfit body", "powerlifter", "strongman", "strong woman", "muscle mommy",
	"muscle daddy", "twunk", "twink", "bear", "otter", "cub", "daddy type",
	"silver fox", "gilded", "distinguished", "weathered", "youthful glow",
	"unblemished", "flawless skin", "porcelain", "alabaster", "sun kissed",
	"bronze", "golden tan", "farmers tan", "tan lines", "no tan lines",
}

// suggestionsHair - Hair colors and styles (~100 terms)
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
	// Additional hair terms
	"silver hair", "grey hair", "white hair", "salt pepper", "frosted tips",
	"teal hair", "orange hair", "yellow hair", "neon hair", "pastel hair",
	"rose gold hair", "burgundy hair", "auburn hair", "chestnut hair",
	"honey blonde", "ash blonde", "golden blonde", "caramel highlights",
	"chocolate brown", "jet black", "blue black", "raven hair", "ebony hair",
	"silky hair", "shiny hair", "glossy hair", "matte hair", "textured hair",
	"kinky hair", "coily hair", "4c hair", "3c hair", "2c hair", "1c hair",
	"heat damaged", "natural texture", "permed", "relaxed", "pressed",
	"blowout", "silk press", "twist out", "braid out", "wash and go",
	"loc extensions", "faux locs", "sisterlocks", "micro locs", "free form locs",
	"finger waves", "marcel waves", "beach waves", "loose curls", "tight curls",
}

// suggestionsAge - Age categories (~80 terms)
var suggestionsAge = []string{
	"18 years old", "19 years old", "20s", "30s", "40s", "50s", "60s", "70s",
	"college", "university", "student", "schoolgirl", "cheerleader",
	"young adult", "middle aged", "older woman", "older man", "age gap",
	"age difference", "barely legal", "fresh 18", "just turned 18", "legal teen",
	"young looking", "mature looking", "ageless", "youthful", "experienced",
	"senior", "elderly", "grandma", "grandpa", "cougar", "sugar daddy",
	"sugar mommy", "older younger", "may december", "generation gap",
	// Additional age terms
	"18 year old", "19 year old", "20 year old", "21 year old", "22 year old",
	"23 year old", "24 year old", "25 year old", "late teens", "early 20s",
	"mid 20s", "late 20s", "early 30s", "mid 30s", "late 30s", "early 40s",
	"mid 40s", "late 40s", "early 50s", "mid 50s", "late 50s", "early 60s",
	"prime", "peak", "in her prime", "in his prime", "coming of age",
	"newly legal", "fresh faced", "baby faced", "looking young", "looking old",
	"aging gracefully", "silver vixen", "silver daddy", "mature babe",
	"mature stud", "experienced lover", "seasoned pro", "veteran performer",
	"newcomer", "newbie", "first timer", "debut", "new to industry",
	"career veteran", "industry legend", "retired comeback", "classic star",
}

// suggestionsSexualActs - Sexual acts (~150 terms)
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
	// Additional sexual acts
	"double blowjob", "triple blowjob", "gloryhole blowjob", "car blowjob",
	"road head", "under table", "sneaky blowjob", "quickie blowjob",
	"slow blowjob", "teasing blowjob", "edging blowjob", "ruined orgasm",
	"post orgasm torture", "forced orgasm", "multiple orgasms", "back to back",
	"creampie eating", "felching", "snowballing", "cum swapping", "cum sharing",
	"spit roast", "airtight", "water tight", "dvp", "double vaginal",
	"triple vaginal", "quadruple penetration", "all holes filled",
	"simultaneous orgasm", "mutual orgasm", "synchronized", "together",
}

// suggestionsFetishes - Fetishes & kinks (~150 terms)
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
	// Additional fetish terms
	"breath control", "choking kink", "hand on throat", "light choking",
	"heavy choking", "passing out", "blackout", "consensual non consent",
	"cnc", "rape fantasy", "ravishment", "forced fantasy", "struggle",
	"resistance play", "take down", "primal", "primal play", "hunter prey",
	"chase", "capture", "predator prey", "animal instincts", "growling",
	"biting", "scratching", "marking", "claiming", "ownership", "possession",
}

// suggestionsScenarios - Scenarios & situations (~150 terms)
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
	// Additional scenario terms
	"gym hookup", "yoga class", "spin class", "crossfit hookup", "trainer client",
	"physical therapy", "sports massage", "deep tissue", "happy ending",
	"full service", "escort service", "call girl", "high class escort",
	"sugar baby", "arrangement", "allowance", "spoiled", "kept woman",
	"mistress affair", "side chick", "side piece", "other woman", "other man",
	"marriage counselor", "couples therapy", "sex therapy", "tantric session",
	"spiritual awakening", "kundalini", "chakra alignment", "energy exchange",
}

// suggestionsPositions - Positions & actions (~120 terms)
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
	// Additional position terms
	"reverse pronebone", "lazy missionary", "coital alignment", "cat position",
	"deck chair", "jockey position", "suspended congress", "splitting bamboo",
	"victory position", "viennese oyster", "wrap around", "the grip",
	"the lock", "the vice", "crab position", "table top", "right angle",
	"plough position", "shoulder stand", "candle position", "head down",
	"ass up", "face in pillow", "clutching sheets", "grabbing headboard",
	"holding ankles", "holding wrists", "pinned down", "pinned up",
	"pressed against", "pushed into", "pulled close", "held tight",
}

// suggestionsProduction - Production & quality (~120 terms)
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
	// Additional production terms
	"dolby vision", "hdr", "hdr10", "hlg", "rec 2020", "wide color gamut",
	"film look", "raw footage", "log footage", "color corrected", "graded",
	"professional lighting", "three point lighting", "rembrandt lighting",
	"butterfly lighting", "loop lighting", "split lighting", "broad lighting",
	"short lighting", "high key", "low key", "silhouette", "backlit",
	"front lit", "side lit", "motivated lighting", "practical lighting",
	"ambient", "available light", "golden hour", "blue hour", "magic hour",
	"studio setting", "location shoot", "on set", "outdoor location",
	"indoor location", "controlled environment", "natural environment",
}

// suggestionsRelationships - Relationship types (~80 terms)
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
	// Additional relationship terms
	"primary partner", "secondary partner", "nesting partner", "anchor partner",
	"satellite partner", "long distance", "ldr", "online relationship",
	"virtual relationship", "pen pal", "met online", "dating site", "hookup app",
	"arranged meeting", "setup", "blind introduction", "mutual friend",
	"work spouse", "office romance", "forbidden office", "secret affair",
	"on again off again", "complicated", "its complicated", "situationship",
	"talking stage", "exclusive", "non exclusive", "mono", "monogamous",
	"monogamish", "solo poly", "kitchen table poly", "parallel poly",
	"hierarchical", "non hierarchical", "egalitarian", "equal partners",
}

// suggestionsLocations - Settings & locations (~150 terms)
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
	// Additional location terms
	"treehouse", "gazebo", "pergola", "deck", "veranda", "porch", "front porch",
	"back porch", "sun room", "conservatory", "wine cellar", "panic room",
	"man cave", "she shed", "home office", "study", "library room", "den",
	"game room", "media room", "home theater", "music room", "art studio",
	"craft room", "workshop", "tool shed", "greenhouse grow room", "indoor pool",
	"olympic pool", "infinity pool", "rooftop pool", "private beach", "nude beach",
	"clothing optional", "resort pool", "water park", "theme park", "amusement park",
	"fair ground", "carnival", "circus", "zoo", "aquarium", "museum",
	"art gallery", "exhibition", "convention center", "trade show", "expo",
}

// suggestionsClothing - Clothing & accessories (~120 terms)
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
	// Additional clothing terms
	"pasties", "nipple covers", "body tape", "fashion tape", "bralette",
	"sports bra", "push up bra", "padded bra", "unpadded bra", "wireless bra",
	"front clasp", "back clasp", "strapless", "halter", "racerback",
	"plunge bra", "balconette", "demi cup", "full cup", "triangle bra",
	"bra and panty set", "matching set", "mismatched", "boy shorts", "hipster",
	"bikini cut", "high waist", "low rise", "mid rise", "cheeky", "full coverage",
	"crotchless", "open cup", "shelf bra", "quarter cup", "peek a boo",
	"cutout", "keyhole", "lace up", "ribbon tie", "bow detail", "ruffle",
}

// suggestionsSpecificActs - Specific acts & details (~120 terms)
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
	// Additional specific acts
	"throat pie", "cum in throat", "cum down throat", "forced swallow",
	"spit or swallow", "cum gargle", "cum bubble", "cum facial drip",
	"cum walk", "public cum walk", "cum on clothes", "cum stain",
	"multiple cumshots", "back to back cumshots", "rapid fire", "reload",
	"second round", "third round", "marathon cumshots", "cum tribute",
	"self facial", "ruined facial", "surprise facial", "unexpected cumshot",
}

// suggestionsNiches - Popular niches (~100 terms)
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
	// Additional niche terms
	"cock rating", "dick rating", "pussy rating", "body rating", "rate me",
	"humiliation joi", "denial joi", "ruined joi", "cei", "cum eating instruction",
	"sph", "small penis humiliation", "praise", "affirmation", "gentle femdom",
	"findom", "financial domination", "pay pig", "money slave", "tribute",
	"goddess worship", "queen worship", "princess treatment", "brat",
	"brat taming", "brat training", "punishment joi", "task", "challenge",
	"dare video", "truth or dare", "spin wheel", "random", "surprise box",
	"mystery content", "grab bag", "bundle deal", "discount", "sale",
}

// suggestionsCombinations - Combinations & modifiers (~80 terms)
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
	// Additional combination terms
	"bwc", "big white cock", "bac", "big asian cock", "blc", "big latino cock",
	"bbbc", "monster cock", "horse cock", "huge cock", "massive dick",
	"thick cock", "fat cock", "long cock", "curved cock", "straight cock",
	"uncut cock", "cut cock", "circumcised", "uncircumcised", "foreskin",
	"veiny cock", "smooth cock", "pretty cock", "ugly cock", "perfect cock",
}

// suggestionsAdditional - Additional common terms (~80 terms)
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
	// Additional common terms
	"strip club", "gentlemans club", "private dance", "champagne room",
	"vip room", "bottle service", "afterparty", "after hours", "late night",
	"early morning", "wake up sex", "morning wood", "sleepy sex", "lazy sex",
	"tired sex", "makeup sex", "angry sex", "hate sex", "love making",
	"first time together", "anniversary sex", "birthday sex", "celebration",
	"holiday sex", "new years", "valentines day", "christmas", "halloween sex",
}

// suggestionsDescriptive - Popular descriptive terms (~80 terms)
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
	// Additional descriptive terms
	"cock hungry", "cum hungry", "attention seeking", "show off",
	"exhibitionist", "shy", "nervous", "confident", "dominant personality",
	"submissive personality", "bratty", "obedient", "defiant", "rebellious",
	"innocent looking", "guilty pleasure", "forbidden fruit", "temptress",
	"seductress", "siren", "enchantress", "vixen", "minx", "tease",
	"heartbreaker", "man eater", "cougar hunter", "milf hunter", "teen lover",
	"older lover", "experience seeker", "adventure seeker", "thrill seeker",
}

// suggestionsAnalContent - Anal content specific (~80 terms)
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
	// Additional anal terms
	"anal destruction", "anal wrecked", "ruined hole", "loose hole",
	"tight hole", "virgin hole", "anal massage", "relaxing anal",
	"sensual anal", "romantic anal", "passionate anal", "loving anal",
	"hate anal", "forced anal fantasy", "anal punishment", "anal reward",
	"anal tease", "anal denial", "anal edging", "anal orgasm only",
	"prostate orgasm", "sissygasm", "handsfree orgasm", "anal only lifestyle",
}

// suggestionsLesbianContent - Lesbian content specific (~80 terms)
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
	// Additional lesbian terms
	"lipstick lesbian", "chapstick lesbian", "stem lesbian", "futch",
	"high femme", "stone butch", "soft butch", "pillow princess", "pillow queen",
	"service top", "power bottom", "verse lesbian", "switch lesbian",
	"lesbian dom", "lesbian sub", "lesbian mistress", "lesbian slave",
	"lesbian pet", "lesbian kitten", "lesbian puppy", "lesbian pony",
	"lesbian worship", "lesbian devotion", "lesbian love story", "lesbian romance",
}

// suggestionsGroupContent - Group content specific (~80 terms)
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
	// Additional group terms
	"train", "running a train", "tag team", "relay", "round robin",
	"all holes", "every hole", "assembly line", "production line",
	"circle jerk", "jack off party", "cum together", "synchronized cum",
	"group masturbation", "mutual group", "watch and jerk", "stroke together",
	"boys night", "girls night", "mixed group", "equal ratio",
	"more girls", "more guys", "solo guy", "solo girl", "center of attention",
	"star of show", "main attraction", "supporting role", "audience participation",
}

// suggestionsBDSMContent - BDSM content specific (~80 terms)
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
	// Additional BDSM terms
	"st andrews cross", "bondage table", "bondage bed", "bondage chair",
	"queening stool", "sybian", "fucking machine", "sex machine",
	"automated", "mechanical", "remote control", "app controlled",
	"long distance control", "public control", "secret vibrator",
	"hidden vibrator", "under clothes", "through clothes", "tease public",
	"forced public", "display", "exhibition bdsm", "show off sub",
}

// suggestionsFetishContent - Specific fetish content (~100 terms)
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
	// Additional fetish terms
	"voice fetish", "accent fetish", "whisper fetish", "moan fetish",
	"laugh fetish", "giggle fetish", "smile fetish", "eye fetish",
	"eye contact fetish", "staring", "watching", "being watched",
	"mirror fetish", "reflection", "self watching", "narcissism",
	"muscle fetish", "flex fetish", "pump fetish", "vascularity",
	"sweat fetish", "gym sweat", "post workout", "glistening",
	"tan fetish", "tan lines fetish", "pale fetish", "contrast",
	"uniform fetish", "costume fetish", "dress up", "transformation fetish",
}

// suggestionsVintageRetro - Vintage and retro content (~80 terms)
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
	// Additional vintage terms
	"film era", "vhs era", "dvd era", "early internet", "dial up era",
	"pre streaming", "mail order", "magazine era", "polaroid era",
	"super 8", "16mm", "35mm", "analog", "before digital", "digitized",
	"restored", "remastered", "upscaled", "ai enhanced", "cleaned up",
	"original quality", "authentic vintage", "period accurate", "era appropriate",
	"costume accurate", "set design", "retro aesthetic", "vintage filter",
	"film grain effect", "vhs effect", "camcorder look", "home video style",
}

// suggestionsTechnology - Technology related (~80 terms)
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
	// Additional technology terms
	"ai generated", "deepfake", "face swap", "body swap", "voice clone",
	"neural network", "machine learning", "computer generated",
	"cgi", "animation", "motion capture", "mocap", "digital human",
	"metaverse", "virtual world", "second life", "vr chat", "social vr",
	"haptic feedback", "teledildonics", "remote pleasure", "synced toys",
	"bluetooth toy", "wifi toy", "internet controlled", "long distance toy",
	"wearable", "discreet wearable", "public wearable", "hands free",
	"app controlled toy", "pattern vibrator", "custom pattern", "music sync",
}

// NEW CATEGORIES

// suggestionsRole - Role-play scenarios (~150 terms)
var suggestionsRole = []string{
	// Professional roles
	"doctor roleplay", "nurse roleplay", "patient roleplay", "medical exam",
	"gynecologist exam", "prostate exam", "physical exam", "check up",
	"teacher roleplay", "student roleplay", "professor roleplay", "tutor roleplay",
	"principal roleplay", "detention", "after school", "private lesson",
	"boss roleplay", "secretary roleplay", "employee roleplay", "job interview roleplay",
	"promotion", "raise negotiation", "workplace affair", "office after hours",
	"police roleplay", "cop roleplay", "arrest roleplay", "interrogation",
	"strip search", "cavity search", "handcuff roleplay", "good cop bad cop",
	"military roleplay", "soldier roleplay", "drill sergeant", "boot camp",
	"navy roleplay", "marine roleplay", "air force", "army roleplay",
	"firefighter roleplay", "rescue fantasy", "save me", "hero roleplay",
	"paramedic roleplay", "ambulance", "emergency room", "trauma center",
	// Service roles
	"maid roleplay", "french maid", "butler roleplay", "servant roleplay",
	"waitress roleplay", "waiter roleplay", "bartender roleplay", "barista",
	"flight attendant", "stewardess", "pilot roleplay", "mile high",
	"hotel staff", "room service roleplay", "concierge", "bellhop",
	"masseuse roleplay", "massage therapist", "happy ending massage", "full service massage",
	"personal trainer roleplay", "gym trainer", "yoga instructor", "fitness coach",
	// Fantasy roles
	"vampire roleplay", "werewolf roleplay", "demon roleplay", "angel roleplay",
	"witch roleplay", "wizard roleplay", "fairy roleplay", "elf roleplay",
	"alien roleplay", "space exploration", "abduction fantasy", "probing",
	"robot roleplay", "android roleplay", "cyborg", "ai companion",
	"superhero roleplay", "villain roleplay", "damsel in distress", "rescue fantasy",
	"princess roleplay", "prince roleplay", "king roleplay", "queen roleplay",
	"knight roleplay", "medieval fantasy", "castle roleplay", "dungeon fantasy",
	// Age play (legal adults only)
	"daddy dom", "mommy dom", "little space", "age regression",
	"babygirl", "babyboy", "princess little", "prince little",
	"caregiver", "nurturing", "protective", "guiding",
	// Animal/pet play
	"puppy roleplay", "kitty roleplay", "bunny roleplay", "pony roleplay",
	"fox roleplay", "wolf roleplay", "cow roleplay", "pig roleplay",
	"pet training", "obedience training", "tricks", "commands",
	"collar and leash", "pet bowl", "pet bed", "kennel time",
	// Stranger scenarios
	"stranger fantasy", "anonymous encounter", "blindfold stranger",
	"gloryhole roleplay", "random hookup", "one time thing",
	"no names", "no talking", "silent stranger", "masked stranger",
}

// suggestionsStudio - Studio/production company terms (~120 terms)
var suggestionsStudio = []string{
	// Major studios
	"brazzers", "reality kings", "bangbros", "naughty america", "digital playground",
	"wicked pictures", "vivid", "private", "evil angel", "jules jordan",
	"blacked", "tushy", "vixen", "deeper", "slayed",
	"pure taboo", "adult time", "girlfriends films", "sweetheart video",
	"girlsway", "all girl massage", "mommys girl", "web young",
	"team skeet", "mofos", "fake hub", "fake taxi official",
	"pornhub premium", "xvideos premium", "xhamster premium",
	"kink", "bound gangbangs", "device bondage", "hogtied",
	"public disgrace", "sex and submission", "the upper floor",
	"everything butt", "whipped ass", "electrosluts", "divine bitches",
	"men in pain", "bound gods", "naked kombat", "butt machine boys",
	// Specialty studios
	"legalporno", "gonzo", "dap nation", "giorgio grandi",
	"rocco siffredi", "manuel ferrara", "lexington steele productions",
	"dogfart network", "interracial pickups", "gloryhole initiations",
	"sean cody", "corbin fisher", "belami", "helix studios",
	"men", "next door studios", "falcon studios", "raging stallion",
	"cockyboys", "lucas entertainment", "treasure island media",
	"transsensual", "grooby", "evil angel trans", "trans angels",
	// Amateur networks
	"exploited college girls", "girls do porn", "backroom casting couch",
	"net video girls", "woodman casting", "private casting x",
	"czech casting", "czech streets", "czech couples", "czech parties",
	"public agent", "fake hostel", "fake hospital", "fake driving school",
	// Cam/clip sites
	"chaturbate official", "myfreecams", "stripchat", "cam4",
	"bongacams", "camsoda", "streamate", "livejasmin",
	"manyvids official", "clips4sale official", "iwantclips", "fancentro",
	"onlyfans creator", "fansly creator", "loyalfans", "just for fans",
	// VR studios
	"naughty america vr", "badoink vr", "wankz vr", "vrbangers",
	"czech vr", "virtual real porn", "sexlikereal", "vr porn official",
	// Regional studios
	"japanese av", "s1", "prestige", "ideapocket", "moodyz",
	"caribbean", "heyzo", "1pondo", "tokyo hot",
	"german scout", "magma film", "inflagranti", "ggg",
	"french connection", "dorcel", "marc dorcel", "jacquie et michel",
	"spanish porn", "cumlouder", "fakings", "torbe",
}

// suggestionsAwards - Award categories (~100 terms)
var suggestionsAwards = []string{
	// AVN Awards
	"avn winner", "avn nominee", "best actress avn", "best actor avn",
	"best new starlet", "best scene", "best oral scene", "best anal scene",
	"best group scene", "best three way", "best double penetration scene",
	"best all girl scene", "best boy girl scene", "best solo scene",
	"best pov scene", "best vignette", "best feature", "best parody",
	"best gonzo", "best oral series", "best anal series", "best interracial",
	"best milf movie", "best mature movie", "best teen movie",
	"performer of year", "female performer of year", "male performer of year",
	"most outrageous scene", "best art direction", "best cinematography",
	// XBIZ Awards
	"xbiz winner", "xbiz nominee", "xbiz performer of year",
	"female performer xbiz", "male performer xbiz", "best actress xbiz",
	"best supporting actress", "best supporting actor", "best new starlet xbiz",
	"best scene xbiz", "best director", "studio of year",
	// XRCO Awards
	"xrco winner", "xrco nominee", "cream of crop", "best actress xrco",
	"best cumpilation", "orgasmic analist", "orgasmic oralist",
	"superslut", "best girl girl", "best threeway xrco",
	// TEA Awards
	"tea winner", "tea nominee", "best trans performer",
	"best trans scene", "best trans newcomer", "fan choice",
	// Fan voted
	"fan award winner", "viewer choice", "audience favorite",
	"most popular", "trending performer", "rising star",
	"breakout performer", "comeback performer", "lifetime achievement",
	// Quality descriptors
	"award winning", "critically acclaimed", "industry recognized",
	"hall of fame", "legend", "icon", "pioneer", "trendsetter",
	"innovator", "game changer", "record breaker", "first ever",
	"most decorated", "multi award", "consecutive winner", "dominant",
}

// suggestionsCosplay - Cosplay/character terms (~150 terms)
var suggestionsCosplay = []string{
	// Anime characters
	"anime cosplay", "hentai cosplay", "sailor moon cosplay", "evangelion cosplay",
	"naruto cosplay", "hinata cosplay", "sakura cosplay", "dragon ball cosplay",
	"bulma cosplay", "android 18 cosplay", "one piece cosplay", "nami cosplay",
	"robin cosplay", "attack on titan cosplay", "mikasa cosplay",
	"my hero academia cosplay", "toga cosplay", "uraraka cosplay",
	"demon slayer cosplay", "nezuko cosplay", "mitsuri cosplay",
	"jujutsu kaisen cosplay", "maki cosplay", "nobara cosplay",
	"spy family cosplay", "yor cosplay", "chainsaw man cosplay",
	"makima cosplay", "power cosplay", "cyberpunk edgerunners", "lucy cosplay",
	// Video game characters
	"video game cosplay", "gaming cosplay", "overwatch cosplay",
	"dva cosplay", "mercy cosplay", "widowmaker cosplay", "tracer cosplay",
	"league of legends cosplay", "lol cosplay", "ahri cosplay", "jinx cosplay",
	"katarina cosplay", "miss fortune cosplay", "evelynn cosplay",
	"final fantasy cosplay", "tifa cosplay", "aerith cosplay", "yuna cosplay",
	"resident evil cosplay", "jill valentine cosplay", "ada wong cosplay",
	"lady dimitrescu cosplay", "mortal kombat cosplay", "kitana cosplay",
	"mileena cosplay", "street fighter cosplay", "chun li cosplay",
	"cammy cosplay", "tomb raider cosplay", "lara croft cosplay",
	"nier automata cosplay", "2b cosplay", "zelda cosplay", "princess zelda",
	"metroid cosplay", "samus cosplay", "zero suit samus",
	"genshin impact cosplay", "ganyu cosplay", "raiden cosplay", "keqing cosplay",
	// Movie/TV characters
	"movie cosplay", "tv cosplay", "superhero cosplay",
	"wonder woman cosplay", "catwoman cosplay", "harley quinn cosplay",
	"black widow cosplay", "scarlet witch cosplay", "jean grey cosplay",
	"mystique cosplay", "storm cosplay", "rogue cosplay",
	"star wars cosplay", "princess leia cosplay", "slave leia",
	"padme cosplay", "rey cosplay", "ahsoka cosplay",
	"star trek cosplay", "orion slave girl", "seven of nine cosplay",
	"game of thrones cosplay", "daenerys cosplay", "cersei cosplay",
	"witcher cosplay", "triss cosplay", "yennefer cosplay",
	// Classic characters
	"schoolgirl cosplay", "cheerleader cosplay", "nurse cosplay",
	"maid cosplay", "bunny girl cosplay", "cat girl cosplay",
	"succubus cosplay", "vampire cosplay", "witch cosplay",
	"fairy cosplay", "elf cosplay", "angel cosplay", "devil cosplay",
	"pirate cosplay", "cowgirl cosplay", "police cosplay", "military cosplay",
	// Accessories
	"wig cosplay", "colored contacts", "costume makeup", "body paint",
	"prosthetics", "fake ears", "fake fangs", "fake wings", "fake tail",
	"prop weapons", "cosplay armor", "cosplay accessories",
}

// suggestionsGaming - Gaming/gamer related (~120 terms)
var suggestionsGaming = []string{
	// Gamer culture
	"gamer girl", "gamer boy", "gaming setup", "streaming setup",
	"twitch thot", "twitch girl", "streamer girl", "streamer boy",
	"egirl", "eboy", "e girl aesthetic", "e boy aesthetic",
	"ahegao face", "ahegao hoodie", "belle delphine style", "gamer girl bath water",
	"gaming chair", "rgb setup", "led lights", "neon gaming room",
	"controller in hand", "playing games", "distracted gaming",
	"rage quit", "victory celebration", "losing bet", "gaming dare",
	// Gaming scenarios
	"strip gaming", "strip fortnite", "strip mario kart", "strip smash bros",
	"loser strips", "winner decides", "gaming punishment", "gaming reward",
	"gaming bet", "if i lose", "if you win", "challenge accepted",
	"gaming marathon", "all nighter", "energy drinks", "gaming snacks",
	"couch co op", "split screen", "local multiplayer", "same room gaming",
	"online gaming", "voice chat", "discord call", "gaming headset",
	"mic muted", "accidentally unmuted", "forgot to end stream",
	// Gaming references
	"press start", "game over", "continue", "new game plus",
	"boss battle", "final boss", "side quest", "main story",
	"level up", "experience points", "achievement unlocked", "trophy earned",
	"easter egg", "hidden level", "secret character", "unlockable",
	"speedrun", "any percent", "100 percent", "glitchless",
	"pvp", "pve", "raid", "dungeon", "loot", "grinding",
	"respawn", "checkpoint", "save point", "quick save",
	// Equipment
	"gaming mouse", "mechanical keyboard", "gaming monitor",
	"ultrawide monitor", "dual monitors", "triple monitors",
	"standing desk", "gaming desk", "cable management",
	"microphone setup", "webcam setup", "face cam", "capture card",
	"vr headset gaming", "motion controls", "haptic suit",
}

// suggestionsSocialMedia - Social media related (~100 terms)
var suggestionsSocialMedia = []string{
	// Platforms
	"onlyfans", "fansly", "fanvue", "manyvids", "clips4sale",
	"pornhub model", "xvideos model", "xhamster model",
	"instagram model", "ig model", "instagram baddie", "ig baddie",
	"tiktok star", "tiktok famous", "tiktok dancer", "tiktok thot",
	"twitter porn", "x rated twitter", "nsfw twitter",
	"reddit model", "reddit amateur", "reddit verified",
	"tumblr porn", "tumblr aesthetic", "snapchat premium",
	"youtube model", "youtube vlogger", "youtube fitness",
	// Content types
	"social media star", "influencer porn", "influencer leaked",
	"content creator", "independent creator", "self produced",
	"fan funded", "tip menu", "custom content", "personalized content",
	"ppv content", "pay per view", "mass dm", "direct message",
	"live stream", "going live", "live show", "live performance",
	"story content", "disappearing content", "24 hour", "limited time",
	"pinned post", "featured content", "highlights", "best of profile",
	// Engagement
	"subscriber special", "fan appreciation", "loyalty reward",
	"milestone celebration", "follower goal", "sub count",
	"tip goal", "spin wheel", "random content", "mystery box",
	"collab", "collaboration", "crossover", "guest appearance",
	"duo content", "duo show", "couple content", "partner content",
	"featuring", "ft", "with", "and", "versus", "vs",
	// Aesthetics
	"aesthetic", "vibe", "mood", "theme", "feed goals",
	"cohesive feed", "grid layout", "color scheme", "filter style",
	"ring light glow", "natural light aesthetic", "golden hour content",
	"bedroom content", "bathroom content", "outdoor content",
	"public content", "risky content", "almost caught",
}

// suggestionsAnime - Anime/hentai specific (~150 terms)
var suggestionsAnime = []string{
	// General anime/hentai
	"hentai", "anime porn", "animated porn", "cartoon porn",
	"doujinshi", "doujin", "manga porn", "ecchi", "eroge",
	"visual novel", "h game", "nukige", "erotic game",
	"japanese animation", "jav anime", "2d", "3d hentai",
	// Art styles
	"anime style", "manga style", "chibi", "moe", "kawaii",
	"realistic anime", "semi realistic", "stylized", "exaggerated",
	"big eyes", "small mouth", "pointy chin", "heart shaped face",
	"colorful hair", "unnatural hair", "gravity defying hair",
	// Character types
	"waifu", "husbando", "best girl", "best boy",
	"tsundere", "yandere", "kuudere", "dandere", "deredere",
	"imouto", "onee san", "onii chan", "senpai", "kouhai",
	"ojou sama", "gyaru", "gal", "kogal", "ganguro",
	"meganekko", "glasses girl", "megane", "four eyes",
	"nekomimi", "cat ears", "inumimi", "dog ears",
	"usagimimi", "bunny ears", "kemonomimi", "animal ears",
	"shrine maiden", "miko", "priestess", "nun anime",
	// Hentai specific
	"tentacle", "tentacle hentai", "tentacle monster", "plant tentacle",
	"monster hentai", "orc hentai", "goblin hentai", "slime hentai",
	"demon hentai", "angel hentai", "succubus hentai", "incubus",
	"futanari", "futa", "futa on female", "futa on male", "futa on futa",
	"trap hentai", "otokonoko", "femboy anime", "crossdress anime",
	"yaoi", "boys love", "bl", "shounen ai", "bara",
	"yuri", "girls love", "gl", "shoujo ai", "lily",
	"netorare", "ntr", "cheating hentai", "cuckolding hentai",
	"netori", "stealing", "mindbreak", "corruption hentai",
	"hypnosis hentai", "mind control hentai", "brainwash",
	"vanilla hentai", "wholesome hentai", "loving hentai", "romantic hentai",
	// Formats
	"uncensored hentai", "censored hentai", "mosaic", "no mosaic",
	"subbed hentai", "dubbed hentai", "raw hentai", "translated",
	"full color", "black and white", "one shot", "series",
	"ova", "ona", "anime episode", "special episode",
}

// suggestionsCountry - Country-specific terms (~150 terms)
var suggestionsCountry = []string{
	// Japanese content
	"jav", "japanese av", "japanese adult video", "japan porn",
	"tokyo porn", "osaka porn", "japanese amateur", "japanese professional",
	"uncensored jav", "censored jav", "jav idol", "av idol",
	"gravure idol", "junior idol", "photobook", "image video",
	"japanese schoolgirl", "jk", "japanese office lady", "ol",
	"japanese housewife", "japanese milf", "japanese mature",
	// European content
	"european porn", "euro porn", "prague porn", "budapest porn",
	"berlin porn", "paris porn", "london porn", "amsterdam porn",
	"czech porn", "hungarian porn", "german porn", "french porn",
	"british porn", "uk porn", "spanish porn", "italian porn",
	"russian porn", "ukrainian porn", "polish porn", "romanian porn",
	"nordic porn", "swedish porn", "danish porn", "norwegian porn",
	"eastern european", "western european", "mediterranean",
	// North American content
	"american porn", "usa porn", "california porn", "florida porn",
	"las vegas porn", "miami porn", "los angeles porn", "la porn",
	"canadian porn", "montreal porn", "toronto porn", "vancouver porn",
	"mexican porn", "mexico city porn", "tijuana porn",
	// South American content
	"brazilian porn", "brazil porn", "rio porn", "sao paulo porn",
	"colombian porn", "medellin porn", "bogota porn",
	"venezuelan porn", "argentinian porn", "buenos aires porn",
	"peruvian porn", "chilean porn", "latin american",
	// Asian content
	"asian porn", "southeast asian", "east asian", "south asian",
	"korean porn", "k porn", "seoul porn", "thai porn", "bangkok porn",
	"filipino porn", "manila porn", "vietnamese porn", "hanoi porn",
	"chinese porn", "hong kong porn", "taiwanese porn", "singaporean porn",
	"indian porn", "desi porn", "bollywood style", "mumbai porn",
	// Middle Eastern content
	"arab porn", "middle eastern", "persian porn", "iranian porn",
	"turkish porn", "istanbul porn", "lebanese porn", "egyptian porn",
	// African content
	"african porn", "south african porn", "nigerian porn", "kenyan porn",
	"ethiopian porn", "north african", "sub saharan",
	// Australian/Oceanian content
	"australian porn", "aussie porn", "sydney porn", "melbourne porn",
	"new zealand porn", "kiwi porn", "pacific islander porn",
}

// suggestionsPhysical - Physical attributes (~150 terms)
var suggestionsPhysical = []string{
	// Height
	"tall woman", "tall girl", "amazon woman", "statuesque",
	"short woman", "short girl", "petite woman", "fun sized",
	"average height", "model height", "basketball player", "volleyball player",
	"tall man", "short man", "average height man", "giant man",
	// Weight
	"skinny woman", "thin woman", "slender", "willowy",
	"average weight", "healthy weight", "normal body", "regular body",
	"thick woman", "thicc woman", "curvy woman", "voluptuous woman",
	"chubby woman", "plump woman", "bbw woman", "ssbbw woman",
	"feedee woman", "gaining woman", "weight gain", "getting bigger",
	// Specific measurements
	"32a", "32b", "32c", "32d", "34a", "34b", "34c", "34d",
	"36c", "36d", "36dd", "38d", "38dd", "huge naturals",
	"a cup", "b cup", "c cup", "d cup", "dd cup", "e cup", "f cup",
	"flat chest", "small chest", "medium chest", "large chest", "huge chest",
	"small waist", "tiny waist", "narrow waist", "wide waist",
	"small hips", "medium hips", "wide hips", "child bearing hips",
	"small ass", "medium ass", "big ass", "huge ass", "massive ass",
	// Skin
	"pale skin", "fair skin", "light skin", "medium skin", "olive skin",
	"tan skin", "brown skin", "dark skin", "ebony skin", "chocolate skin",
	"caramel skin", "honey skin", "golden skin", "bronze skin",
	"freckled skin", "spotted", "birthmarked", "beauty marks",
	"clear skin", "acne", "scars", "stretch marks", "cellulite",
	"smooth skin", "silky skin", "soft skin", "rough skin",
	"oily skin", "dry skin", "combination skin", "normal skin",
	// Eyes
	"blue eyes", "green eyes", "brown eyes", "hazel eyes", "grey eyes",
	"amber eyes", "violet eyes", "heterochromia", "two different eyes",
	"big eyes", "small eyes", "almond eyes", "round eyes", "hooded eyes",
	"monolid", "double eyelid", "cat eyes", "doe eyes", "bedroom eyes",
	// Lips
	"full lips", "thin lips", "medium lips", "pouty lips", "bee stung lips",
	"natural lips", "lip filler", "enhanced lips", "plump lips",
	// Other
	"long neck", "short neck", "slender neck", "thick neck",
	"strong jaw", "weak jaw", "square jaw", "pointed chin", "round chin",
	"high cheekbones", "hollow cheeks", "chubby cheeks", "dimpled cheeks",
}

// suggestionsEmotional - Emotional/mood descriptors (~100 terms)
var suggestionsEmotional = []string{
	// Positive emotions
	"happy", "joyful", "ecstatic", "blissful", "euphoric",
	"excited", "enthusiastic", "eager", "willing", "ready",
	"loving", "affectionate", "tender", "caring", "nurturing",
	"passionate", "intense", "fervent", "ardent", "burning",
	"playful", "teasing", "flirty", "coy", "mischievous",
	"confident", "bold", "assertive", "commanding", "powerful",
	"relaxed", "calm", "serene", "peaceful", "tranquil",
	// Arousal states
	"horny", "turned on", "aroused", "stimulated", "excited sexually",
	"wet", "dripping", "soaking", "throbbing", "aching",
	"needy", "desperate", "craving", "hungry for", "starving for",
	"insatiable", "unquenchable", "relentless", "tireless", "endless",
	// Submission/dominance emotions
	"submissive mood", "obedient", "compliant", "yielding", "surrendering",
	"dominant mood", "controlling", "commanding", "authoritative", "powerful",
	"bratty", "defiant", "resistant", "challenging", "testing",
	// Vulnerability
	"shy", "nervous", "anxious", "hesitant", "uncertain",
	"vulnerable", "exposed", "open", "trusting", "giving",
	"embarrassed", "blushing", "flustered", "overwhelmed", "overstimulated",
	// Intensity
	"intense", "extreme", "overwhelming", "mind blowing", "earth shattering",
	"gentle", "soft", "tender", "delicate", "careful",
	"rough mood", "aggressive", "primal", "animalistic", "savage",
	// Connection
	"intimate", "connected", "bonded", "united", "one",
	"romantic", "loving connection", "deep connection", "soul connection",
	"casual", "no strings", "just physical", "purely sexual",
}

// suggestionsTime - Time/duration related (~80 terms)
var suggestionsTime = []string{
	// Duration
	"quickie", "quick fuck", "fast sex", "5 minute", "10 minute",
	"15 minute", "20 minute", "30 minute", "45 minute", "1 hour",
	"hour long", "extended", "marathon", "all night", "all day",
	"multiple rounds", "round two", "round three", "encore",
	"short clip", "medium length", "long video", "full length",
	"compilation long", "supercut", "mega compilation",
	// Time of day
	"morning sex", "morning wood", "wake up sex", "breakfast in bed",
	"afternoon delight", "lunch break", "midday", "siesta sex",
	"evening sex", "after work", "dinner date", "nightcap",
	"late night", "midnight", "after midnight", "early morning",
	"sunrise", "sunset", "golden hour", "blue hour",
	// Frequency
	"daily", "weekly", "monthly", "regular", "frequent",
	"occasional", "rare", "special occasion", "anniversary",
	"first time", "second time", "hundredth time", "lost count",
	"once a day", "twice a day", "multiple times", "cant stop",
	// Timing
	"premature", "quick finish", "fast cummer", "one minute man",
	"edging", "delayed", "prolonged", "extended foreplay",
	"simultaneous", "together", "at same time", "synchronized",
	"sequential", "one after another", "back to back", "non stop",
	// Era
	"modern", "contemporary", "current", "recent", "new",
	"classic era", "golden era", "vintage era", "retro era",
}

// suggestionsTrending - Trending/viral terms (~100 terms)
var suggestionsTrending = []string{
	// Current trends
	"trending", "viral", "going viral", "blowing up", "everywhere",
	"popular right now", "hot right now", "current favorite", "todays best",
	"weekly top", "monthly best", "yearly favorite", "all time best",
	"most watched", "most liked", "most shared", "most commented",
	"highest rated", "five stars", "perfect score", "top rated",
	// Social trends
	"challenge", "viral challenge", "tiktok challenge", "trend challenge",
	"trying trend", "following trend", "joining trend", "participating",
	"hashtag", "trending hashtag", "viral hashtag", "popular hashtag",
	"meme", "viral meme", "current meme", "trending meme",
	// Content trends
	"reaction", "reaction video", "try not to", "challenge video",
	"pov trend", "asmr trend", "roleplay trend", "cosplay trend",
	"outfit trend", "makeup trend", "hair trend", "aesthetic trend",
	"minimalist trend", "maximalist trend", "cottagecore", "dark academia",
	// Performance trends
	"new position", "trending position", "popular position", "viral position",
	"new technique", "trending technique", "popular technique",
	"new style", "trending style", "popular style", "current style",
	// Discovery
	"new find", "hidden gem", "underrated", "slept on",
	"up and coming", "rising star", "breakout", "newcomer",
	"discovery", "found", "stumbled upon", "accidentally found",
	"recommended", "algorithm", "for you", "suggested",
	"similar to", "if you liked", "fans also watched", "related",
	// Engagement
	"must watch", "cant miss", "essential viewing", "required",
	"bookmark", "save for later", "add to playlist", "favorite",
	"share worthy", "send to friend", "screenshot", "screen record",
}

// SearchSuggestions contains comprehensive adult content search terms (5000+ terms)
// Built into the binary at compile time for privacy and performance
// Organized by category for easier maintenance
var SearchSuggestions = func() []string {
	// Pre-allocate with estimated capacity
	all := make([]string, 0, 8000)

	// Original categories
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

	// New categories
	all = append(all, suggestionsRole...)
	all = append(all, suggestionsStudio...)
	all = append(all, suggestionsAwards...)
	all = append(all, suggestionsCosplay...)
	all = append(all, suggestionsGaming...)
	all = append(all, suggestionsSocialMedia...)
	all = append(all, suggestionsAnime...)
	all = append(all, suggestionsCountry...)
	all = append(all, suggestionsPhysical...)
	all = append(all, suggestionsEmotional...)
	all = append(all, suggestionsTime...)
	all = append(all, suggestionsTrending...)

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
		{
			Name:        "roleplay",
			DisplayName: "Role Play",
			Terms:       []string{"nurse", "teacher", "maid", "police", "doctor", "secretary", "cheerleader", "schoolgirl"},
		},
		{
			Name:        "anime",
			DisplayName: "Anime/Hentai",
			Terms:       []string{"hentai", "anime", "cosplay", "ahegao", "tentacle", "futanari", "yuri", "yaoi"},
		},
		{
			Name:        "production",
			DisplayName: "Production",
			Terms:       []string{"brazzers", "reality kings", "bangbros", "blacked", "tushy", "vixen", "kink", "evil angel"},
		},
		{
			Name:        "trending",
			DisplayName: "Trending",
			Terms:       []string{"viral", "trending", "popular", "top rated", "most watched", "new", "onlyfans", "tiktok"},
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
