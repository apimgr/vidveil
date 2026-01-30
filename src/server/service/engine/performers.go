// SPDX-License-Identifier: MIT
package engine

import (
	"sort"
	"strings"
)

// Performer data organized by category for maintainability
// Each slice contains names relevant to that category
// Combined into Performers at init time

// performersTier1 - Most searched performers (industry recognized top performers)
var performersTier1 = []string{
	"mia khalifa", "lana rhoades", "riley reid", "abella danger", "angela white",
	"adriana chechik", "emily willis", "gabbie carter", "eva elfie", "autumn falls",
	"elsa jean", "mia malkova", "kendra lust", "brandi love", "lisa ann",
	"nicole aniston", "alexis texas", "madison ivy", "asa akira", "sasha grey",
	"jenna jameson", "christy mack", "dani daniels", "lexi belle", "kagney linn karter",
	"phoenix marie", "keisha grey", "valentina nappi", "gianna michaels", "sara jay",
	"alexis fawx", "julia ann", "syren de mer", "cory chase", "cherie deville",
	"india summer", "reagan foxx", "dee williams", "kit mercer", "london river",
	"lena paul", "violet myers", "skylar vox", "natasha nice", "kenzie reeves",
	"gina valentina", "karlee grey", "jill kassidy", "gia derza", "maya bijou",
	"jane wilde", "vina sky", "kira noir", "ana foxxx", "daya knight",
	"scarlit scandal", "jenna foxx", "september reign", "misty stone", "chanell heart",
	"kendra sunderland", "blair williams", "jessa rhodes", "carter cruise", "aj applegate",
	"kelsi monroe", "anikka albrite", "remy lacroix", "tori black", "kayden kross",
	"jesse jane", "stoya", "leah gotti", "august ames", "peta jensen",
	"anissa kate", "aletta ocean", "jasmine jae", "johnny sins", "manuel ferrara",
	"xander corvus", "keiran lee", "ramon nomar", "mick blue", "markus dupree",
	"dredd", "jax slayher", "isiah maxwell", "rob piper", "ricky johnson",
	"small hands", "tyler nixon", "seth gamble", "ryan mclane", "chad white",
	"damon dice", "lucas frost", "codey steele", "danny d", "jordi el nino polla",
}

// performersTier2 - Very popular performers
var performersTier2 = []string{
	"brooklyn chase", "julia ann", "cherie deville", "silvia saige", "mercedes carrera",
	"richelle ryan", "sheena ryder", "ryan keely", "pristine edge", "katie morgan",
	"brittany andrews", "daisy stone", "layla london", "bridgette b", "ariella ferrera",
	"ava addams", "nikki benz", "jewels jade", "diamond foxxx", "veronica avluv",
	"dana dearmond", "tanya tate", "danica dillon", "kianna dior", "jessica jaymes",
	"shay fox", "alura jenson", "karen fisher", "samantha 38g", "minka",
	"rachael cavalli", "joslyn james", "holly halston", "charlee chase", "isis love",
	"esperanza gomez", "franceska jaimes", "canela skin", "katana kombat", "serena santos",
	"victoria june", "monica asis", "sophia leone", "aaliyah hadid", "luna star",
	"rose monroe", "kitty caprice", "juliana vega", "diamond kitty", "lela star",
	"marica hase", "london keyes", "alina li", "cindy starfall", "jade kush",
	"ayumi anime", "kendra spade", "ember snow", "morgan lee", "kalina ryu",
	"mia li", "sharon lee", "rae lil black", "polly pons", "may thai",
	"hitomi tanaka", "julia", "eimi fukada", "yua mikami", "anri okita",
	"aika", "ai uehara", "tia tanaka", "asa akira", "katsuni",
	"skin diamond", "diamond jackson", "anya ivy", "teanna trump", "moriah mills",
	"brittney white", "skyler nicole", "nadia jay", "yasmine de leon", "layton benton",
	"harley dean", "jezabel vessir", "raven redmond", "sarah banks", "nicole bexley",
	"nicole doshi", "vicki chase", "cassie del isla", "sofia rose", "penny barber",
	"joanna angel", "arabelle raphael", "siri dahl", "katrina jade", "jynx maze",
}

// performersMilf - MILF/Mature performers
var performersMilf = []string{
	"ava addams", "ariella ferrera", "nikki benz", "bridgette b", "jewels jade",
	"diamond foxxx", "veronica avluv", "dana dearmond", "tanya tate", "danica dillon",
	"mercedes carrera", "kianna dior", "jessica jaymes", "shay fox", "alura jenson",
	"karen fisher", "samantha 38g", "kayla kleevage", "claudia marie", "minka",
	"dee williams", "rachael cavalli", "joslyn james", "holly halston", "charlee chase",
	"brandi love", "kendra lust", "lisa ann", "julia ann", "cory chase",
	"cherie deville", "india summer", "reagan foxx", "kit mercer", "london river",
	"syren de mer", "alexis fawx", "brooklyn chase", "silvia saige", "sheena ryder",
	"ryan keely", "pristine edge", "katie morgan", "brittany andrews", "richelle ryan",
	"nina hartley", "deauxma", "sally dangelo", "erica lauren", "rita daniels",
	"persia monir", "kelly madison", "devon lee", "ava devine", "holly sampson",
	"eva notty", "sybil stallone", "alana cruise", "amber jayne", "becky bandini",
	"mona wales", "christie stevens", "natasha starr", "krissy lynn", "tara holiday",
	"rayveness", "shayla laveaux", "darla crane", "janet mason", "zoey holloway",
	"simone sonay", "mellanie monroe", "allison moore", "julia ann", "claudia valentine",
	"diana prince", "sophia lomeli", "shayla leveaux", "tanya james", "alana evans",
	"kristal summers", "joclyn stone", "raylene", "jennifer best", "inari vachs",
	"marie mccray", "bibette blanche", "vanessa videl", "anjelica lauren", "kylie ireland",
	"lacey duvalle", "teri weigel", "dyanna lauren", "darla crane", "lisa lipps",
	"kiara mia", "claudia kealoha", "vanilla deville", "sienna west", "mya diamond",
	"leena sky", "shyla stylez", "madison monroe", "viana milian", "katee owen",
	"mandy flores", "cali carter", "penny pax", "casey calvert", "aiden starr",
	"nina elle", "dava foxx", "brooklyn lee", "diamond simone", "alyssa lynn",
	"sarah vandella", "ivy lebelle", "veronica vain", "lilian stone", "linzee ryder",
	"angie noir", "nina north", "tiffany mynx", "kaylani lei", "sienna west",
}

// performersTeen - Teen/Young/College performers
var performersTeen = []string{
	"kenzie madison", "lulu chu", "chloe cherry", "haley reed", "harmony wonder",
	"alex coal", "nia nacci", "kylie rocket", "lacy lennon", "paige owens",
	"emma hix", "athena faris", "angel smalls", "piper perri", "riley star",
	"kristen scott", "alex grey", "lily rader", "naomi swann", "aria lee",
	"brooklyn gray", "jazmin luv", "xxlayna marie", "maya woulfe", "lily larimar",
	"eliza ibarra", "vanna bardot", "sky bri", "olivia madison", "gracie may green",
	"bella rose", "dillion harper", "dakota skye", "elsa jean", "jessa rhodes",
	"aidra fox", "ariana marie", "melissa moore", "megan rain", "karla kush",
	"valentina nappi", "dani daniels", "amirah adara", "mia split", "pepper xo",
	"jasmine grey", "macy meadows", "alina lopez", "jane rogers", "kali roses",
	"scarlet skies", "nikki sweet", "kira perez", "maya kendrick", "lilly bell",
	"ana rose", "alex blake", "jaye summers", "haley spades", "aria banks",
	"megan marx", "maddy may", "olive glass", "sophia lux", "vienna rose",
	"emma starletto", "rebel rhyder", "addison ryder", "avery cristy", "bunny colby",
	"dolly leigh", "elena koshka", "flora cherimoya", "goldie rush", "harmony rivers",
	"ivy aura", "jojo kiss", "kyler quinn", "lexi lore", "maya farrell",
	"natalie brooks", "paisley porter", "quinn wilde", "reagan lush", "skyler storm",
	"tiffany tatum", "una fairy", "venus vixen", "willa prescott", "ximena garcia",
	"yasmin scott", "zoe bloom", "adria rae", "bailey brooke", "cali carter",
	"daisy drew", "eden ivy", "fiona frost", "gwen vicious", "hanna hawthorne",
	"izzy belle", "jade nile", "kenna james", "luna mills", "morgan rain",
	"natasha vilanova", "ophelia kaan", "phoebe cakes", "quinn cummings", "raven redmond",
	"selena stone", "taylor sands", "ursula x", "veronica dean", "willow ryder",
	"xianna hill", "yuki love", "zelda morrison", "alice pink", "briar rose",
	"carolina sweets", "delilah day", "evelyn claire", "felicity feline", "gianna gem",
	"holly hendrix", "iris ivy", "jessie saint", "katalina kyle", "lily lou",
	"melody marks", "nadia noja", "olga snow", "paisley rae", "quinn waters",
	"riley anne", "sabina rouge", "theodora day", "una alexandar", "violet rain",
	"whitney wright", "xeena mae", "yuri honma", "zara durose", "anastasia knight",
}

// performersAsian - Asian performers
var performersAsian = []string{
	"marica hase", "london keyes", "alina li", "cindy starfall", "jade kush",
	"ayumi anime", "kendra spade", "ember snow", "morgan lee", "kalina ryu",
	"mia li", "sharon lee", "rae lil black", "polly pons", "may thai",
	"asia akira", "hitomi tanaka", "julia", "eimi fukada", "yua mikami",
	"anri okita", "aika", "ai uehara", "tia tanaka", "katsuni",
	"miko lee", "kaylani lei", "tera patrick", "priya rai", "jessica bangkok",
	"jayden lee", "charmane star", "kobe tai", "fujiko kano", "kira kener",
	"mika tan", "evelyn lin", "christine lee", "nadia ali", "lana violet",
	"layla lei", "gaia", "kaho shibuya", "yuu shinoda", "rion nishikawa",
	"jessica kizaki", "sakura sakurada", "mao hamasaki", "shunka ayami", "minami kojima",
	"kirara asuka", "tsubasa amami", "jessica kizaki", "asahi mizuno", "kokomi sakura",
	"rina ishihara", "ai sayama", "yui hatano", "momo sakura", "haruki sato",
	"rui hasegawa", "nami hoshino", "mion sonoda", "ruka kanae", "eri hosaka",
	"miho ichiki", "sora shiina", "ayaka tomoda", "yuki jin", "ami tokita",
	"miku ohashi", "remu suzumori", "mina kitano", "rina ueda", "anju mizushima",
	"luna star", "lulu kinouchi", "mei matsumoto", "nao jinguji", "ria sakuragi",
	"saeko matsushita", "sena minami", "shoko takahashi", "tsubomi", "waka ninomiya",
	"yuki jin", "yume takeda", "yuria satomi", "emi fukada", "maria ozawa",
	"sora aoi", "saori hara", "hana haruna", "mika kayama", "rio hamasaki",
	"mihiro taniguchi", "minami aoyama", "reiko kobayakawa", "ruri saijo", "hitomi",
	"kanna hashimoto", "koharu suzuki", "mao kurata", "miku airi", "mitsuki nagisa",
	"nanami kawakami", "nono mizusawa", "remu hayami", "shiori tsukada", "yuki kashiwagi",
	"belle amore", "kate mori", "lena reif", "mia park", "nyla thai",
	"onna mae", "penny pax", "suki sin", "vivian lang", "yuki mori",
	"ami emerson", "bruce venture", "celia taylor", "diana doll", "ella nova",
	"francesca le", "ginger lynn", "holly mae", "isabella nice", "jasmine chen",
	"kelly divine", "lara jade", "mika kim", "nadia styles", "ophelia bloom",
	"piper fawn", "quinn quest", "renae cruz", "sienna lee", "tiffany rain",
	"daisy ducati", "katya clover", "anya krey", "saya song", "vicki chase",
}

// performersLatina - Latina performers
var performersLatina = []string{
	"luna star", "rose monroe", "kitty caprice", "juliana vega", "diamond kitty",
	"lela star", "esperanza gomez", "franceska jaimes", "canela skin", "katana kombat",
	"serena santos", "victoria june", "monica asis", "sophia leone", "aaliyah hadid",
	"madison ivy", "gia vendetti", "vanessa sky", "ginebra bellucci", "susy gala",
	"apolonia lapiedra", "vicki chase", "cassie del isla", "abella danger", "autumn falls",
	"maya bijou", "ember snow", "veronica rodriguez", "mercedes carrera", "veronica leal",
	"andreina deluxe", "arianna jay", "ayana angel", "bianca breeze", "brianna love",
	"carla cruz", "daisy marie", "eva angelina", "francesca le", "gabby quinteros",
	"hailey havoc", "isabella nice", "jynx maze", "karla lane", "lissa love",
	"mia martinez", "nadia styles", "olivia o lovely", "penny flame", "rebeca linares",
	"sabrina suzuki", "tatiana kush", "valeria blue", "wendy moon", "ximena lucero",
	"yenny contreras", "zafira", "ada sanchez", "briana banderas", "cinthia doll",
	"destiny love", "eva saldana", "felicity feline", "gina torres", "holly hendrix",
	"izzy bell", "jada stevens", "kesha ortega", "liv wild", "mandy muse",
	"natalia starr", "ocean pearl", "penelope reed", "quinn quest", "rosalie ruiz",
	"sara luvv", "tia cyrus", "uma jolie", "vienna black", "wicked witch",
	"xiomara cruz", "yara skye", "zaya cassidy", "adriana maya", "bella reese",
	"carmen caliente", "diana prince", "elena rivera", "frida sante", "giselle leon",
	"havana bleu", "iris rose", "jazmin torres", "kendra cole", "luna corazon",
	"melissa moore", "natalie monroe", "olivia wilder", "priya price", "quinn kennedy",
	"remy lacroix", "sasha de sade", "tina kay", "una alexandar", "valentina jewels",
	"wanda nara", "xiomara cruz", "yara skye", "zaya cassidy", "abigail mac",
	"bianca beauchamp", "catalina cruz", "dani jensen", "evie delatosso", "francesca jaimes",
	"giselle mari", "harley jade", "isabella gonzalez", "jada cruz", "kayla carrera",
	"lana roy", "maya hills", "nina daniels", "olivia austin", "peta jensen",
	"quinn kennedy", "rosie munroe", "sasha rose", "tiffany torres", "uma jolie",
}

// performersEbony - Ebony performers
var performersEbony = []string{
	"jenna foxx", "chanell heart", "diamond jackson", "anya ivy", "teanna trump",
	"moriah mills", "brittney white", "skyler nicole", "nadia jay", "yasmine de leon",
	"layton benton", "harley dean", "skin diamond", "jezabel vessir", "raven redmond",
	"sarah banks", "nicole bexley", "osa lovely", "nikki darling", "ashley pink",
	"kira noir", "ana foxxx", "daya knight", "scarlit scandal", "september reign",
	"misty stone", "jada fire", "naomi banxx", "lacey duvalle", "marie luv",
	"kapri styles", "jada stevens", "aryana adin", "bria myles", "cassidy banks",
	"demi sutra", "ebony mystique", "fallen angel", "gabby quinteros", "honey gold",
	"ivy sherwood", "julie kay", "katt dylan", "loni legend", "maserati",
	"nina devon", "olivia jayy", "phoenix marie", "queen rogue", "raven hart",
	"sarai minx", "tori montana", "unique", "verta", "willow ryder",
	"xena love", "yara skye", "zoey reyes", "adriana maya", "big beautiful",
	"cali caliente", "diamond monrow", "ebony goddess", "flora cherimoya", "gabriela lopez",
	"hazel grace", "india westbrooks", "jada doll", "kiki minaj", "lisa tiffian",
	"mya mays", "nina rotti", "onyx rose", "paris milan", "queen jezabel",
	"royalty black", "sophia fiore", "tiana rose", "unique pleasure", "veronique vega",
	"wanda curtis", "ximena gomez", "yanna lavish", "zara carter", "asia rae",
	"bria backwoods", "cleo gold", "daizy cooper", "ebony elegance", "faith black",
	"gina valentina", "havana ginger", "imani rose", "jade jordan", "kylie luxxx",
	"leilani leeane", "melody black", "nyomi banxxx", "olivia love", "princess paris",
	"raylene rivers", "sabrina banks", "tyler faith", "unique starr", "victoria cakes",
	"wendy williams", "xana star", "yaya mai", "zariah june", "aaliyah love",
	"bethany benz", "chanell heart", "dolly diore", "erika vution", "gala brown",
	"harmony vision", "indigo vanity", "jade aspen", "kamille amora", "lotus lain",
	"maserati xxx", "nia nacci", "opal dream", "piper jones", "queenie sateen",
}

// performersEuropean - European performers
var performersEuropean = []string{
	"anna de ville", "alysa gap", "nataly gold", "francesca le", "proxy paige",
	"casey calvert", "bonnie rotten", "katrina jade", "jynx maze", "holly michaels",
	"lily love", "casey cumz", "raven bay", "marley brinx", "stella cox",
	"sienna day", "ella hughes", "carmel anderson", "rebecca more", "tina kay",
	"sybil stallone", "liya silver", "nancy ace", "anna polina", "anita bellini",
	"aletta ocean", "jasmine jae", "tiffany doll", "clea gaultier", "anny aurora",
	"nathaly cherie", "jenny wild", "nicole love", "sarah sultry", "victoria pure",
	"katy rose", "claudia macc", "angelica heart", "ania kinski", "alexis crystal",
	"amber jayne", "blanche bradburry", "cayenne klein", "dolly leigh", "eufrat",
	"foxy di", "george uhl", "henessy", "ivana sugar", "jenny hard",
	"katarina muti", "lena paul", "mia melone", "natalie cherie", "olivia grace",
	"paulina soul", "queensnake", "rosalina love", "sophia laure", "taissia shanti",
	"uliana", "victoria sweet", "wendy moon", "xena", "yiki", "zafira",
	"adriana chechik", "bella baby", "cathy heaven", "denise sky", "elena koshka",
	"florane russell", "ginebra bellucci", "helena moeller", "ilona fox", "jolee love",
	"katana kombat", "linda leclair", "misha cross", "nikita bellucci", "olga barz",
	"patty michova", "queenie sateen", "rosella visconti", "silvia dellai", "tiffany tatum",
	"ukranian babe", "valentina ricci", "wanda nara", "xara", "yarina", "zoe doll",
	"alice romain", "blue angel", "carol ferrer", "diana dali", "emma fantazy",
	"francys belle", "ginger fox", "helena kim", "isabella clark", "julia de lucia",
	"kathia nobili", "lara latex", "maria fiori", "naomi bennet", "ornella morgan",
	"penelope cum", "queen paris", "rebecca volpetti", "sandra romain", "taylee wood",
	"uma zex", "verona sky", "whitney conroy", "xiomara cruz", "yolanda ortega",
	"zazie skymm", "amirah adara", "barbara bieber", "cherry kiss", "dolly diore",
	"elena vega", "frida sante", "gina devine", "helen star", "irina vega",
	"josephine jackson", "katy jayne", "lexi dona", "mary rock", "nicole black",
}

// performersMale - Male performers
var performersMale = []string{
	"johnny sins", "manuel ferrara", "xander corvus", "keiran lee", "ramon nomar",
	"mick blue", "markus dupree", "dredd", "jax slayher", "isiah maxwell",
	"rob piper", "ricky johnson", "small hands", "tyler nixon", "seth gamble",
	"ryan mclane", "chad white", "damon dice", "lucas frost", "codey steele",
	"danny d", "jordi el nino polla", "rocco siffredi", "evan stone", "james deen",
	"tommy gunn", "john strong", "erik everhard", "steve holmes", "nacho vidal",
	"marco banderas", "christian xxx", "derrick pierce", "bill bailey", "michael vegas",
	"chris strokes", "alex legend", "jay smooth", "sean michaels", "lexington steele",
	"mandingo", "prince yashua", "jon jon", "rico strong", "flash brown",
	"jason brown", "sean lawless", "jmac", "brick danger", "tyler steel",
	"logan pierce", "bruce venture", "van wylde", "tony rubino", "bambino",
	"zac wild", "oliver flynn", "scott nails", "charles dera", "ryan ryans",
	"jay snake", "david perry", "renato", "alberto blanco", "potro de bilbao",
	"nick moreno", "jordi enp", "brian pumper", "mr pete", "lee stone",
	"dale dabone", "rocco reed", "marcus london", "richie calhoun", "tommy pistol",
	"peter north", "ron jeremy", "evan stone", "randy spears", "mark wood",
	"marco ducati", "ryan driller", "will havoc", "danny mountain", "alan stafford",
	"jake adams", "kyle mason", "jake jace", "johnny love", "dillon harper",
	"seth gamble", "mickey mod", "keni styles", "tony brooklyn", "brad hart",
	"ramon nomar", "toni ribas", "max deeds", "dorian del isla", "victor solo",
	"ian scott", "mike adriano", "chris diamond", "nick ross", "jason luv",
	"slim poke", "jack napier", "prince yahshua", "flash brown", "jovan jordan",
	"oliver davis", "dante colle", "casey jacks", "rico marlon", "jason steel",
	"jay rock", "chad diamond", "jack blaque", "ace rockwood", "lucky b",
	"danny steele", "nathan bronson", "jay romero", "robby echo", "stirling cooper",
}

// performersClassic - Classic/Vintage performers
var performersClassic = []string{
	"nina hartley", "christy canyon", "ginger lynn", "amber lynn", "traci lords",
	"hyapatia lee", "kay parker", "marilyn chambers", "annette haven", "seka",
	"lisa de leeuw", "juliet anderson", "vanessa del rio", "sharon mitchell", "veronica hart",
	"jamie summers", "bunny bleu", "gail palmer", "jessica wylde", "rachel ashley",
	"angel kelly", "keisha", "barbi benton", "becky savage", "brigitte aime",
	"candy samples", "desiree cousteau", "erica boyer", "francois papillon", "georgina spelvin",
	"honey wilder", "jennifer welles", "kathy hartley", "loni sanders", "marlene willoughby",
	"nikki sinn", "ona zee", "penny morgan", "raven richards", "samantha fox",
	"teri weigel", "uschi digard", "veronica vera", "wanda curtis", "xaviera hollander",
	"yolanda", "zara whites", "asia carrera", "briana banks", "chasey lain",
	"devon", "elizabeth x", "felicia tang", "gauge", "holly body",
	"jenteal", "kaitlyn ashley", "letha weapons", "midori", "nikita denise",
	"olivia del rio", "penelope pumpkins", "raylene", "savannah", "tera patrick",
	"ursula cavalcanti", "valentina", "wanda nara", "xtc", "yolanda fox",
	"jenna jameson", "jill kelly", "julia ann", "kobe tai", "krystal steal",
	"lexus", "miko lee", "nikki nova", "olivia", "phoenix ray",
	"roxy jezel", "shyla stylez", "taylor rain", "utopia", "victoria paris",
	"wendy divine", "xena", "yasmine bleeth", "zara whites", "asia akira",
	"brittany oconnell", "chloe jones", "dru berrymore", "eve lawrence", "faye reagan",
	"gianna", "hillary scott", "isabella soprano", "jana cova", "kaylan nicole",
	"lela star", "mika tan", "nautica thorn", "olivia olovely", "penny flame",
	"rachel starr", "savanna samson", "tiffany holiday", "uma stone", "velicity von",
	"wendy james", "xiomara", "yolanda", "zara", "adriana sage",
	"breanne benson", "cassidey", "dylan ryder", "eva angelina", "flower tucci",
}

// performersRecent - Recent/Emerging performers (2020+)
var performersRecent = []string{
	"sky bri", "molly little", "kazumi", "blake blossom", "savannah bond",
	"liz jordan", "anastasia brokelyn", "coco lovelock", "madi collins", "angel youngs",
	"aria valencia", "babi star", "brookie blair", "clara trinity", "diana grace",
	"eva maxim", "freya mayer", "gizelle blanco", "hime marie", "indica flower",
	"jazlyn ray", "kayley gunner", "lily starfire", "macy meadows", "natalie knight",
	"octavia red", "paisley paige", "queenie sateen", "reagan foxx", "serena hill",
	"theodora day", "una fairy", "val dodds", "wren walker", "xeena mae",
	"yolanda ortega", "zaya cassidy", "alex jones", "bibi noel", "caitlin bell",
	"dana dearmond", "ellie eilish", "fifi foxx", "gianna grey", "halle hayes",
	"isla summer", "jia lissa", "kiara cole", "lana rose", "madelyn monroe",
	"nataly gold", "olive glass", "penny archer", "quinn quest", "rharri rhound",
	"sophia burns", "tru kait", "una alexandar", "venus vixen", "willow ryder",
	"xaya lovelle", "yuki love", "zoe sparx", "aiden ashley", "bunny madison",
	"charlie forde", "diana prince", "ella hollywood", "fionna", "gabbie hanna",
	"hazel moore", "ivy wolfe", "jana jordan", "kat dior", "lily labeau",
	"mia linz", "nikole nash", "olivia lua", "phoenix madina", "queenie sateen",
	"romi rain", "sara may", "tiffany watson", "una stone", "vera king",
	"whitney westgate", "xana star", "yara skye", "zee twins", "adira allure",
	"bella rolland", "carmen valentina", "daisy stone", "ember lace", "flora cherimoya",
	"grace noel", "hannah jo", "iris ivy", "jayde symz", "khloe kapri",
	"lacey starr", "maddison lee", "nikita von james", "osa lovely", "pristine edge",
	"quincy", "raven rockette", "serene siren", "tiffany fox", "unique starr",
	"valentina nappi", "wendy williams", "xianna hill", "yesenia rock", "zaya cassidy",
}

// performersInternational - International performers (various countries)
var performersInternational = []string{
	"alexis crystal", "anissa kate", "anna polina", "anny aurora", "apolonia lapiedra",
	"athina", "carol ferrer", "clea gaultier", "dolly leigh", "elena koshka",
	"florane russell", "ginebra bellucci", "henessy", "isabella clark", "jenny wild",
	"katarina muti", "liya silver", "mia melone", "nancy ace", "nathaly cherie",
	"nicole love", "paulina soul", "rosalina love", "sarah sultry", "silvia dellai",
	"sofia cucci", "sophia laure", "taissia shanti", "tiffany doll", "uliana",
	"valentina ricci", "verona sky", "victoria pure", "victoria sweet", "wendy moon",
	"blue angel", "carol ferrer", "denisa peterson", "diana dali", "eveline dellai",
	"francys belle", "gina devine", "helena kim", "irina vega", "josephine jackson",
	"kathia nobili", "lexi dona", "maria fiori", "mary rock", "naomi bennet",
	"ornella morgan", "penelope cum", "rebecca volpetti", "sandra romain", "taylee wood",
	"uma zex", "verona sky", "whitney conroy", "zazie skymm", "amirah adara",
	"barbara bieber", "cherry kiss", "dolly diore", "elena vega", "frida sante",
	"helen star", "irina vega", "jolee love", "katarina muti", "linda leclair",
	"misha cross", "nikita bellucci", "olga barz", "patty michova", "queenie sateen",
	"rosella visconti", "silvia dellai", "tiffany tatum", "valentina ricci", "wanda nara",
	"yarina", "zoe doll", "alice romain", "emma fantazy", "francys belle",
	"ginger fox", "helena kim", "isabella clark", "julia de lucia", "kathia nobili",
	"lara latex", "maria fiori", "naomi bennet", "ornella morgan", "penelope cum",
	"queen paris", "rebecca volpetti", "sandra romain", "taylee wood", "uma zex",
	"canela skin", "cassie del isla", "ginebra bellucci", "mona kim", "susan ayn",
	"vicky love", "wanessa boyer", "ximena lucero", "yenifer chacon", "zafira",
	"alessandra jane", "brenda boop", "candice demellza", "daniella margot", "elena rivera",
	"fiby", "gabriella paltrova", "helena price", "iveta", "julia roca",
}

// performersAlt - Alternative/tattooed/goth/punk performers
var performersAlt = []string{
	"joanna angel", "arabelle raphael", "small hands", "christy mack", "bonnie rotten",
	"anna bell peaks", "katrina jade", "ivy lebelle", "leigh raven", "nikki hearts",
	"draven star", "jessie lee", "vera drake", "carmen caliente", "xander corvus",
	"rocky emerson", "rizzo ford", "ophelia rain", "genevieve sinn", "kissa sins",
	"karma rx", "sheridan love", "siri dahl", "sydney cole", "payton preslee",
	"tana lea", "lydia black", "jade baker", "amber ivy", "cadey mercury",
	"amarna miller", "anna deville", "audrey noir", "cali carter", "charlotte sartre",
	"dahlia sky", "demi sutra", "eleanor", "faye reagan", "gigi allens",
	"harlow harrison", "holly hendrix", "ivy aura", "janice griffith", "juelz ventura",
	"kelly divine", "kylie ireland", "lena kelly", "lola fae", "maitland ward",
	"nadia styles", "nikki delano", "olivia kasady", "penny pax", "rachael madori",
	"raven bay", "romi rain", "sasha de sade", "scarlet de sade", "skin diamond",
	"stoya", "sydney cole", "tori avano", "veronica rose", "violet monroe",
	"xo gisele", "zelda morrison", "aiden starr", "bree daniels", "coral aorta",
	"dana vespoli", "eloa lombard", "felix jones", "gwen stark", "halsey rae",
	"indigo augustine", "janey doe", "kelly stafford", "leya falcon", "marsha may",
	"nikki sexx", "olive glass", "proxy paige", "rizzo ford", "samantha bentley",
	"tanya tate", "ultimate surrender", "valentina jewels", "wendy moon", "xander corvus",
	"yuki mori", "zarrah angel", "alexxa vice", "bella vendetta", "coral snake",
	"delirious hunter", "ember snow", "freya parker", "gina snake", "harley quinn",
	"iggy amore", "jessie volt", "kelly shibari", "lizz tayler", "malena morgan",
	"nikki benz", "orion starr", "princess donna", "queens landing", "redhead redemption",
}

// performersTransgender - Transgender performers
var performersTransgender = []string{
	"natalie mars", "eva maxim", "ella hollywood", "chanel santini", "venus lux",
	"aubrey kate", "daisy taylor", "casey kisses", "bailey jay", "sarina valentina",
	"korra del rio", "kimber haven", "marissa minx", "lena kelly", "jane marie",
	"jessy dubai", "sofia sanders", "izzy wilde", "melanie brooks", "kayleigh coxx",
	"jenna gargles", "kenzie taylor", "nikki vicious", "ariel demure", "foxxy",
	"domino presley", "mia isabella", "nina lawless", "chelsea poe", "jade venus",
	"shiri allwood", "zariah aura", "ts madison", "aspen brooks", "mandy mitchell",
	"khloe kay", "lianna lawson", "shiri trapman", "nadia love", "angelina please",
	"rubi maxim", "bambi prescott", "emma rose", "crystal thayer", "yasmin lee",
	"vaniity", "morgan bailey", "jonelle brooks", "tiffany starr", "jessica fox",
	"annalise rose", "kylie maria", "alexa scout", "kimber lee", "mara nova",
	"sadie hawkins", "jamie french", "danielle foxx", "mickaela", "tori mayes",
	"trans performer", "amanda jade", "carmen moore", "danni daniels", "eva lin",
	"foxxy", "grooby girl", "honey foxxx", "isabella sorrenti", "jane marie",
	"kendra sinclaire", "lina cavalli", "mandy mitchell", "nadia love", "olivia love",
	"penny tyler", "queenie sateen", "rosa velvet", "sasha de sade", "tiffany starr",
	"uma jolie", "vanessa jhons", "wendy summers", "xena", "yasmin lee",
}

// Performers is the combined list of all performers from all categories
var Performers = func() []string {
	// Pre-allocate with estimated capacity
	all := make([]string, 0, 3500)

	all = append(all, performersTier1...)
	all = append(all, performersTier2...)
	all = append(all, performersMilf...)
	all = append(all, performersTeen...)
	all = append(all, performersAsian...)
	all = append(all, performersLatina...)
	all = append(all, performersEbony...)
	all = append(all, performersEuropean...)
	all = append(all, performersMale...)
	all = append(all, performersClassic...)
	all = append(all, performersRecent...)
	all = append(all, performersInternational...)
	all = append(all, performersAlt...)
	all = append(all, performersTransgender...)

	// Deduplicate
	seen := make(map[string]bool, len(all))
	unique := make([]string, 0, len(all))
	for _, name := range all {
		nameLower := strings.ToLower(strings.TrimSpace(name))
		if nameLower != "" && !seen[nameLower] {
			seen[nameLower] = true
			unique = append(unique, name)
		}
	}

	return unique
}()

// PerformerSuggestion represents a performer autocomplete suggestion
type PerformerSuggestion struct {
	Name  string `json:"name"`
	Score int    `json:"-"`
}

// AutocompletePerformers returns performer suggestions for a partial name
func AutocompletePerformers(prefix string, maxResults int) []PerformerSuggestion {
	if prefix == "" || maxResults <= 0 {
		return nil
	}

	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if len(prefix) < 2 {
		return nil
	}

	var suggestions []PerformerSuggestion

	for _, name := range Performers {
		nameLower := strings.ToLower(name)

		score := 0
		if strings.HasPrefix(nameLower, prefix) {
			// Exact prefix match - highest score
			score = 100 - len(name)
		} else {
			// Check each word in the name
			words := strings.Fields(nameLower)
			for _, word := range words {
				if strings.HasPrefix(word, prefix) {
					score = 80 - len(name)
					break
				}
			}
			// Contains match - lower score
			if score == 0 && strings.Contains(nameLower, prefix) {
				score = 50 - len(name)
			}
		}

		if score > 0 {
			suggestions = append(suggestions, PerformerSuggestion{
				Name:  name,
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

// GetPopularPerformers returns a list of popular performers
func GetPopularPerformers(count int) []string {
	// Return the top performers (first in list = most popular)
	if count > len(Performers) {
		count = len(Performers)
	}
	if count > 20 {
		count = 20 // Cap at 20 for popular list
	}
	return Performers[:count]
}
