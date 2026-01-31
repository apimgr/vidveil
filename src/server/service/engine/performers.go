// SPDX-License-Identifier: MIT
package engine

import (
	"sort"
	"strings"
)

// Performer data organized by category for maintainability
// Each slice contains names relevant to that category
// Combined into Performers at init time
// Total unique performers: 3000+

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
	"valentina jewels", "skylar snow", "emma starletto", "aria lee", "lily larimar",
	"athena faris", "vanna bardot", "eliza ibarra", "maya woulfe", "brooklyn gray",
	"jazmin luv", "xxlayna marie", "olive glass", "sophia lux", "vienna rose",
	"rebel rhyder", "addison ryder", "avery cristy", "bunny colby", "dolly leigh",
	"elena koshka", "flora cherimoya", "goldie rush", "harmony rivers", "ivy aura",
	"anna claire clouds", "blake blossom", "alyx star", "gianna dior", "vince karter",
	"casey calvert", "chanel camryn", "maitland ward", "gal ritchie", "alex knight",
	"luna star", "kazumi", "emily lynne", "iggy azalea", "seth gamble",
}

// performersTier2 - Very popular performers
var performersTier2 = []string{
	"brooklyn chase", "silvia saige", "mercedes carrera",
	"richelle ryan", "sheena ryder", "ryan keely", "pristine edge", "katie morgan",
	"brittany andrews", "daisy stone", "layla london", "bridgette b", "ariella ferrera",
	"ava addams", "nikki benz", "jewels jade", "diamond foxxx", "veronica avluv",
	"dana dearmond", "tanya tate", "danica dillon", "kianna dior", "jessica jaymes",
	"shay fox", "alura jenson", "karen fisher", "samantha 38g", "minka",
	"rachael cavalli", "joslyn james", "holly halston", "charlee chase", "isis love",
	"esperanza gomez", "franceska jaimes", "canela skin", "katana kombat", "serena santos",
	"victoria june", "monica asis", "sophia leone", "aaliyah hadid",
	"rose monroe", "kitty caprice", "juliana vega", "diamond kitty", "lela star",
	"marica hase", "london keyes", "alina li", "cindy starfall", "jade kush",
	"ayumi anime", "kendra spade", "ember snow", "morgan lee", "kalina ryu",
	"mia li", "sharon lee", "rae lil black", "polly pons", "may thai",
	"hitomi tanaka", "julia", "eimi fukada", "yua mikami", "anri okita",
	"aika", "ai uehara", "tia tanaka", "katsuni",
	"skin diamond", "diamond jackson", "anya ivy", "teanna trump", "moriah mills",
	"brittney white", "skyler nicole", "nadia jay", "yasmine de leon", "layton benton",
	"harley dean", "jezabel vessir", "raven redmond", "sarah banks", "nicole bexley",
	"nicole doshi", "vicki chase", "cassie del isla", "sofia rose", "penny barber",
	"joanna angel", "arabelle raphael", "siri dahl", "katrina jade", "jynx maze",
	"lilly bell", "kenzie anne", "kylie rocket",
	"lulu chu", "maddy may", "madi collins", "minxx marley", "nala nova",
	"natalia nix", "nicole aria", "paisley porter", "quinn wilde", "reagan lush",
	"siri", "sky bri", "spencer bradley", "tommy king", "vanessa cage",
	"karma rx", "romi rain", "chanel preston", "alena croft", "lauren phillips",
	"jasmine webb", "cecilia lion", "sarah jessie", "kimmy granger", "naomi woods",
	"molly jane", "cadence lux", "lena anderson", "aften opal", "leana lovings",
	"alex coal", "paisley paige", "scarlett sage", "kate dalia", "jazlyn ray",
	"haley spades", "jade venus", "jewelz blu", "lily lou", "theodora day",
	"natalie knight", "catalina ossa", "alexis tae", "laney grey", "freya parker",
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
	"simone sonay", "mellanie monroe", "allison moore", "claudia valentine",
	"diana prince", "sophia lomeli", "tanya james", "alana evans",
	"kristal summers", "joclyn stone", "raylene", "jennifer best", "inari vachs",
	"marie mccray", "bibette blanche", "vanessa videl", "anjelica lauren", "kylie ireland",
	"lacey duvalle", "teri weigel", "dyanna lauren", "lisa lipps",
	"kiara mia", "claudia kealoha", "vanilla deville", "sienna west", "mya diamond",
	"leena sky", "shyla stylez", "madison monroe", "viana milian",
	"mandy flores", "cali carter", "penny pax", "casey calvert", "aiden starr",
	"nina elle", "dava foxx", "brooklyn lee", "diamond simone", "alyssa lynn",
	"sarah vandella", "ivy lebelle", "veronica vain", "lilian stone", "linzee ryder",
	"angie noir", "nina north", "tiffany mynx", "kaylani lei",
	"sara st clair", "jessie lee", "alyssa reece", "kathy anderson", "shalina devine",
	"leigh darby", "gia paige", "emma starr", "michelle wild",
	"savannah jane", "carmen valentina", "sabrina cyns", "kiki daire", "darcie dolce",
	"tara ashley", "kylie kingston", "kate linn", "artemisia love", "mona azar",
	"kailani kai", "casca akashova", "penny archer", "lexi luna",
	"anissa kate", "sensual jane", "eva karera", "candy manson", "isis taylor",
	"veronica rayne", "hunter bryce", "kinzie kenner", "sindy lange", "diamond jackson",
	"monique alexander", "desi dalton", "jazmyn", "roxanne hall", "michelle lay",
	"tabitha stevens", "sunset thomas", "sindee coxx", "lexi carrington", "amber michaels",
	"brittany oneil", "desi foxx", "tiffany million", "missy monroe", "austin taylor",
	"austin kincaid", "kristina cross", "mikki lynn", "sammie sparks", "taylor wane",
	"sara stone", "darla crane", "vicky vette", "houston", "tabitha stern",
	"wanda lust", "sophia mounds", "persia pele", "raquel devine", "anita cannibal",
	"kelly leigh", "lexi lamour", "daryl hannah", "beverly lynne", "kayla synz",
	"eva angelina", "flower tucci", "sunny lane", "gianna lynn", "brianna beach",
}

// performersTeen - Teen/Young/College performers (18+)
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
	"selena stone", "taylor sands", "veronica dean", "willow ryder",
	"xianna hill", "yuki love", "zelda morrison", "alice pink", "briar rose",
	"carolina sweets", "delilah day", "evelyn claire", "felicity feline", "gianna gem",
	"holly hendrix", "iris ivy", "jessie saint", "katalina kyle", "lily lou",
	"melody marks", "nadia noja", "olga snow", "paisley rae", "quinn waters",
	"riley anne", "sabina rouge", "theodora day", "violet rain",
	"whitney wright", "zara durose", "anastasia knight",
	"spencer bradley", "octavia red", "rhiannon ryder", "hime marie", "sera ryder",
	"jewelz blu", "kate quinn", "arietta adams", "savannah sixx", "violet starr",
	"demi hawks", "jessa blue", "lexi grey", "molly little", "natalie porkman",
	"gal ritchie", "chanel camryn", "aften opal", "leana lovings", "freya parker",
	"alexis tae", "laney grey", "catalina ossa", "kate dalia", "jazlyn ray",
	"indica flower", "jade venus", "honey gold", "cecilia lion", "sofi ryan",
	"emma rosie", "bambii bonsai", "liz jordan", "kayley gunner", "coco lovelock",
	"kylie quinn", "mackenzie moss", "sera ryder", "destiny cruz", "demi sutra",
	"destiny diaz", "serena hill", "sky pierce", "stevie moon", "xianna hill",
	"vanessa moon", "tallie lorain", "riley star", "mazy myers", "mariah leonne",
	"maddi winters", "kyra rose", "kiley jay", "jessae rosae", "jasmine wilde",
	"hollie mack", "hannah hays", "gina gerson", "dixie lynn", "cleo vixen",
	"claire black", "cassidy klein", "audrey grace", "aria sky", "angel youngs",
}

// performersAsian - Asian performers
var performersAsian = []string{
	"marica hase", "london keyes", "alina li", "cindy starfall", "jade kush",
	"ayumi anime", "kendra spade", "ember snow", "morgan lee", "kalina ryu",
	"mia li", "sharon lee", "rae lil black", "polly pons", "may thai",
	"asa akira", "hitomi tanaka", "julia", "eimi fukada", "yua mikami",
	"anri okita", "aika", "ai uehara", "tia tanaka", "katsuni",
	"miko lee", "kaylani lei", "tera patrick", "priya rai", "jessica bangkok",
	"jayden lee", "charmane star", "kobe tai", "fujiko kano", "kira kener",
	"mika tan", "evelyn lin", "christine lee", "nadia ali", "lana violet",
	"layla lei", "gaia", "kaho shibuya", "yuu shinoda", "rion nishikawa",
	"jessica kizaki", "sakura sakurada", "mao hamasaki", "shunka ayami", "minami kojima",
	"kirara asuka", "tsubasa amami", "asahi mizuno", "kokomi sakura",
	"rina ishihara", "ai sayama", "yui hatano", "momo sakura", "haruki sato",
	"rui hasegawa", "nami hoshino", "mion sonoda", "ruka kanae", "eri hosaka",
	"miho ichiki", "sora shiina", "ayaka tomoda", "yuki jin", "ami tokita",
	"miku ohashi", "remu suzumori", "mina kitano", "rina ueda", "anju mizushima",
	"lulu kinouchi", "mei matsumoto", "nao jinguji", "ria sakuragi",
	"saeko matsushita", "sena minami", "shoko takahashi", "tsubomi", "waka ninomiya",
	"yume takeda", "yuria satomi", "maria ozawa",
	"sora aoi", "saori hara", "hana haruna", "mika kayama", "rio hamasaki",
	"mihiro taniguchi", "minami aoyama", "reiko kobayakawa", "ruri saijo", "hitomi",
	"kanna hashimoto", "koharu suzuki", "mao kurata", "miku airi", "mitsuki nagisa",
	"nanami kawakami", "nono mizusawa", "remu hayami", "shiori tsukada", "yuki kashiwagi",
	"belle amore", "kate mori", "lena reif", "mia park", "nyla thai",
	"onna mae", "suki sin", "vivian lang", "yuki mori",
	"ami emerson", "celia taylor", "diana doll", "ella nova",
	"lara jade", "mika kim", "nadia styles", "ophelia bloom",
	"piper fawn", "quinn quest", "renae cruz", "sienna lee", "tiffany rain",
	"daisy ducati", "katya clover", "anya krey", "saya song", "vicki chase",
	"avery black", "clara trinity", "gia oh my", "june liu", "kimmy kimm",
	"lola fae", "mia lelani", "nicole doshi", "tiffany doll",
	"yiming curiosity", "akira shell", "asia zo", "bambii bonsai", "cece stone",
	"diana thai", "elle lee", "faye lau", "hana yuki",
	"iris lei", "kira kosarin", "lana lei", "mia le",
	"ocha thai", "pui yi", "queen yasmine", "rina ellis",
	"sakura sena", "tia ling", "uma jolie", "vicky chai", "wasa yatai",
	"xia xiang", "yuki sakura", "zuki", "akemi hoshi", "bella lei",
	"nicole doshi", "lulu chu", "luna mills", "jade venus", "lena moon",
	"asian persuasion", "asia carrera", "miko sinz", "lea hart", "nari park",
	"mila jade", "asia zo", "bella ling", "cici rhodes", "lee", "meiko",
	"connie carter", "evelyn lin", "gaia", "honey moon", "ivory may",
	"jade sin", "kai lee", "lani lane", "maya shin", "nene yoshitaka",
	"olvia thai", "pon", "rina rina", "saya soto", "tara thai",
	"yuki mori", "ada sanchez", "brenna sparks", "chanel lee", "danielle wang",
	"eva yi", "frances chung", "gina kim", "haruki satou", "isabella pak",
	"jenny yoo", "kathleen tanaka", "lisa wu", "maria chang", "niki lee young",
	"olivia cheng", "priya price", "quinn lee", "rina umemiya", "sachiko",
	"tanya song", "uma thai", "violet ling", "wendy wang", "xena thai",
	"yuki sakura", "zoe tang", "ann li", "brittney white",
	"chyna doll", "deborah song", "evelyn rey", "fiona cheeks", "gloria gucci",
	"heather lee", "ivana thai", "jessica lee", "kris lee", "loni legend",
	"monica mayhem", "nancy ho", "olivia thai", "pebbles", "queenie chen",
	"rosario stone", "suki", "tiffany preston", "umi hirose", "victoria rae",
	"wendy fiore", "xena kai", "yoyo chan", "zara white",
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
	"destiny love", "eva saldana", "gina torres", "holly hendrix",
	"izzy bell", "jada stevens", "kesha ortega", "liv wild", "mandy muse",
	"natalia starr", "ocean pearl", "penelope reed", "rosalie ruiz",
	"sara luvv", "tia cyrus", "vienna black",
	"xiomara cruz", "yara skye", "zaya cassidy", "adriana maya", "bella reese",
	"carmen caliente", "elena rivera", "frida sante", "giselle leon",
	"havana bleu", "iris rose", "jazmin torres", "kendra cole", "luna corazon",
	"melissa moore", "natalie monroe", "olivia wilder", "priya price", "quinn kennedy",
	"remy lacroix", "sasha de sade", "tina kay",
	"bianca beauchamp", "catalina cruz", "dani jensen", "evie delatosso",
	"giselle mari", "harley jade", "isabella gonzalez", "jada cruz", "kayla carrera",
	"lana roy", "maya hills", "nina daniels", "olivia austin", "peta jensen",
	"rosie munroe", "sasha rose", "tiffany torres",
	"alina belle", "carolina abril", "gabriela paltrova", "hazel heart",
	"isabella soprano", "jolla rosa", "kristina rose", "luna lovely", "maya morena",
	"natalia love", "olivia olovely", "paula shy", "rosa bloom",
	"talia mint", "vanessa luna", "xiomara", "yara martinez", "zoey foxx",
	"amara romani", "brianna banks", "carla brasil", "dulce", "esmeralda",
	"fernanda", "gloria", "helena", "iris", "jasmine",
	"katia", "lorena", "monica", "nadia", "olga",
	"patricia", "rosario", "sonia", "tanya",
	"ursula", "valentina", "wilma", "yolanda", "zoe",
	"mercedes lynn", "penelope cross", "selena castro", "ariella ferrera", "sophia fiore",
	"isabella de santos", "claudia valentine", "lupe fuentes", "alexis love", "jasmine byrne",
	"isis love", "jessie rogers", "jenna presley", "missy martinez", "raven bay",
	"ruby rayes", "sandy sweet", "sienna west", "valerie kay", "ximena navarrete",
	"adriana deville", "belladonna", "candy martinez", "delilah strong", "eva notty",
	"gina lynn", "havana ginger", "isabella cruz", "jaylene rio", "kaiya lynn",
	"lorena sanchez", "mercedes ashley", "nadia ali", "olivia del rio", "priya anjali rai",
	"rebeca linares", "sativa rose", "tiffany taylor", "valerie herrera", "vicky vette",
	"wendy breeze", "yarissa duran", "zafira", "alexa nicole", "bibi jones",
	"carolina sweets", "diana prince", "estrella", "felony", "gina valentina",
	"havana bleu", "izzy bella", "jada stevens", "kissa sins", "lola foxx",
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
	"demi sutra", "ebony mystique", "honey gold",
	"ivy sherwood", "julie kay", "katt dylan", "loni legend", "maserati",
	"nina devon", "olivia jayy", "queen rogue", "raven hart",
	"sarai minx", "tori montana", "verta", "willow ryder",
	"xena love", "yara skye", "zoey reyes", "adriana maya",
	"cali caliente", "diamond monrow", "flora cherimoya", "gabriela lopez",
	"hazel grace", "india westbrooks", "jada doll", "kiki minaj", "lisa tiffian",
	"mya mays", "nina rotti", "onyx rose", "paris milan",
	"royalty black", "sophia fiore", "tiana rose",
	"wanda curtis", "ximena gomez", "yanna lavish", "zara carter", "asia rae",
	"bria backwoods", "cleo gold", "daizy cooper", "faith black",
	"havana ginger", "imani rose", "jade jordan", "kylie luxxx",
	"leilani leeane", "melody black", "nyomi banxxx", "olivia love", "princess paris",
	"raylene rivers", "sabrina banks", "tyler faith",
	"wendy williams", "xana star", "yaya mai", "zariah june", "aaliyah love",
	"bethany benz", "dolly diore", "erika vution", "gala brown",
	"harmony vision", "indigo vanity", "jade aspen", "kamille amora", "lotus lain",
	"maserati xxx", "nia nacci", "opal dream", "piper jones", "queenie sateen",
	"amari gold", "bree daniels", "coco jay", "destiny diaz",
	"faith leon", "giselle palmer", "hazel may", "ivy aura", "jazzy jamison",
	"kali dreams", "lala ivey", "maya bijou", "nadia ali", "onyx",
	"peaches", "raven swallowz", "simone styles", "tiffany nunez",
	"vanessa monet", "winter jade", "amber stars",
	"chanell", "ebony princess", "foxy brown", "georgia jones", "honey",
	"anya olsen", "bethany benz", "chanell heart", "destiny", "ebony beauty",
	"free", "giselle", "harley", "imani", "jayla",
	"karma", "lexi", "maya", "nova", "osa",
	"princess", "queenie", "raven", "simone", "tiana",
	"unique", "venus", "willow", "xena", "yolanda", "zara",
	"lexington steele", "mandingo", "prince yahshua", "rico strong", "sean michaels",
	"flash brown", "jason brown", "jon jon", "jovan jordan", "mr marcus",
	"janae foxx", "jnayla foxx", "kelly starr", "kina kai", "lacey london",
	"lala ivey", "mocha menage", "monique", "nadia styles", "naomi shaw",
	"promise", "roxy reynolds", "sinnamon love", "skyy black", "stacey cash",
	"stacie lane", "sweet sable", "taylor layne", "tiny star", "vivica",
	"yummy", "adora", "anita blue", "caramel kitten", "chocolate drop",
	"destiny lane", "diva divine", "ebony ayes", "envy star", "fox",
	"heaven", "ice la fox", "janet jacme", "jazmine cashmere", "jezebel",
	"joyful", "karma may", "katt garcia", "kenya", "kianna jayde",
	"lacy green", "layla monroe", "lexxxi lockhart", "lickity lex", "lisa young",
	"lotus lain", "melody nakai", "missy maze", "mocha delight", "nadia hilton",
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
	"patty michova", "rosella visconti", "silvia dellai", "tiffany tatum",
	"valentina ricci", "wanda nara", "xara", "yarina", "zoe doll",
	"alice romain", "blue angel", "carol ferrer", "diana dali", "emma fantazy",
	"francys belle", "ginger fox", "helena kim", "isabella clark", "julia de lucia",
	"kathia nobili", "lara latex", "maria fiori", "naomi bennet", "ornella morgan",
	"penelope cum", "queen paris", "rebecca volpetti", "sandra romain", "taylee wood",
	"uma zex", "verona sky", "whitney conroy", "yolanda ortega",
	"zazie skymm", "amirah adara", "barbara bieber", "cherry kiss", "dolly diore",
	"elena vega", "frida sante", "gina devine", "helen star", "irina vega",
	"josephine jackson", "katy jayne", "lexi dona", "mary rock", "nicole black",
	"alyssa divine", "briana banks", "cristina miller", "diana doll", "emma butt",
	"gina wild", "hannah shaw", "ivana fukalot",
	"kristina", "lucy li", "mia manarote", "nelly", "olga",
	"petra", "renata", "sasha rose", "tanya",
	"ulyana", "vera", "wanda", "xana", "zara",
	"angelika grays", "bridgette", "clara", "dominica", "eva",
	"flora", "greta", "helena", "ingrid",
	"rocco siffredi", "nacho vidal", "steve holmes", "david perry", "thomas stone",
	"leny ewil", "franco roccaforte", "erik everhard", "mike angelo", "omar galanti",
	"andrea dipre", "antonio ross", "bruno sx", "cristian devil", "dino bravo",
	"franco trentalance", "gianluca rocco", "ivan rodriguez", "luca ferrero", "marco banderas",
	"max cortes", "nick lang", "roberto malone", "silvio evangelista", "tony carrera",
	"kayla green", "kendra star", "lucie wilde", "mea melone", "nekane",
	"rachel evans", "regina ice", "sensual jane", "shione cooper", "silvia saint",
	"sophie moone", "stacy silver", "susana spears", "terry nova", "vanessa decker",
	"wiska", "alexis crystal", "casey jordan", "cayla lyons", "daisy lee",
	"dominica phoenix", "eveline neill", "florane russell", "giorgia roma", "grace joy",
	"heidi hills", "ivana gita", "jenny fer", "jenny saphire", "kira queen",
	"kristy lust", "lucy heart", "mandy dee", "nessa devil", "nikki dream",
	"paula shy", "piper perri", "rachel adjani", "roxy mendez", "sasha sparrow",
	"shalina divine", "shrima malati", "sybil", "timea bella", "vanda lust",
	"victoria daniels", "vinna reed", "vittoria risi", "yasmine gold", "zabava",
}

// performersMale - Male performers
var performersMale = []string{
	"johnny sins", "manuel ferrara", "xander corvus", "keiran lee", "ramon nomar",
	"mick blue", "markus dupree", "dredd", "jax slayher", "isiah maxwell",
	"rob piper", "ricky johnson", "small hands", "tyler nixon", "seth gamble",
	"ryan mclane", "chad white", "damon dice", "lucas frost", "codey steele",
	"danny d", "jordi el nino polla", "james deen", "bruce venture", "evan stone",
	"tommy gunn", "rocco siffredi", "nacho vidal", "lexington steele", "mandingo",
	"prince yahshua", "rico strong", "sean michaels", "shane diesel", "flash brown",
	"jason brown", "jon jon", "jovan jordan", "mr marcus", "rico dawg",
	"chris strokes", "charles dera", "derrick pierce", "erik everhard", "jay smooth",
	"john strong", "karl toughlove", "marco banderas", "michael vegas", "mike adriano",
	"nick manning", "peter north", "ralph long", "ryan driller", "steve holmes",
	"tony de sergio", "voodoo", "will powers", "alex legend", "bambino",
	"brad knight", "chad alva", "duncan saint", "eric john", "filthy rich",
	"jake adams", "johnny castle", "kyle mason", "logan pierce", "marcus london",
	"nathan bronson", "owen gray", "robby echo", "stirling cooper", "tommy pistol",
	"van wylde", "zac wild", "alex jones", "brick danger",
	"dan damage", "ed powers", "franco forte", "gabriel dalessandro",
	"ivan", "jmac", "kai taylor", "lance hart", "mickey mod",
	"nomar", "oliver davis", "porno dan", "quinton james", "ramon",
	"sam shock", "tony rubino", "vinny", "woody johnson",
	"anthony rosano", "bill bailey", "christian", "david perry", "eric masterson",
	"frank major", "guy", "jack hammer",
	"kurt lockwood", "lucky benton", "mark wood", "nick moreno", "oliver strokes",
	"paulo sergio", "rick masters", "sascha", "tee reel",
	"victor solo", "will tile", "xavier corday", "yuri", "zane",
	"andrew stark", "claudio", "daniel hunter",
	"fernando", "george", "harry", "ian", "jack",
	"vince karter", "alex knight", "jay romero", "dante colle", "roman todd",
	"pierce paris", "michael boston", "diego sans", "william seed", "paddy obrien",
	"casey everett", "colby keller", "connor maguire", "damien crosse", "finn harding",
	"griffin barrows", "jessy ares", "johnny rapid", "leo fuentes", "max carter",
	"rafael alencar", "rocco steele", "sean cody", "vadim black", "wolf hudson",
	"brent corrigan", "chris rockway", "diesel washington", "francois sagat", "mason star",
	"pierre fitch", "tyler saint", "zeb atlas", "austin wolf", "boomer banks",
	"bryan hawn", "cliff jensen", "dean monroe", "eric videos", "flex",
	"gage weston", "hunter page", "johnny venture", "kris anderson", "lance luciano",
	"matthew rush", "nick sterling", "olivier robert", "paul fresh", "race cooper",
	"samuel otoole", "topher dimaggio", "trenton ducati", "tyler morgan", "viktor rom",
	"alex mecum", "billy santoro", "cade maddox", "dallas steele", "derek bolt",
}

// performersClassic - Classic/Vintage performers
var performersClassic = []string{
	"jenna jameson", "tera patrick", "christy canyon", "traci lords", "ginger lynn",
	"ashlyn gere", "asia carrera", "janine lindemulder", "jill kelly", "julia ann",
	"kobe tai", "lisa ann", "nikki dial", "peter north", "rocco siffredi",
	"ron jeremy", "selena steele", "shanna mccullough", "stephanie swift", "teri weigel",
	"tori welles", "houston", "brianna banks", "devon", "jesse jane",
	"kayden kross", "kaylani lei", "sunrise adams", "teagan presley", "tory lane",
	"carmella bing", "eva angelina", "gianna michaels", "audrey bitoni", "bree olson",
	"jada fire", "kapri styles", "lacey duvalle", "marie luv", "pinky",
	"nina hartley", "vanessa del rio", "seka", "marilyn chambers", "linda lovelace",
	"amber lynn", "barbara dare", "colleen brennan", "desiree cousteau",
	"georgina spelvin", "holly body", "hyapatia lee", "jamie gillis", "john holmes",
	"kay parker", "leslie bovee", "lisa deleeuw", "loni sanders", "marc wallice",
	"misty beethoven", "paul thomas", "randy spears", "rick savage", "sharon kane",
	"tabitha stevens", "taylor rain", "veronica hart", "victoria paris",
	"wanda curtis", "wendy whoppers", "alexis amore", "angel dark", "aria giovanni",
	"brooke banner", "catalina cruz", "daisy marie", "emily marilyn", "eva lawrence",
	"felecia", "flora", "gauge", "gina lynn", "harmony",
	"india", "isis nile", "jade marcela", "jasmin st claire", "jeanna fine",
	"kylie ireland", "melissa lauren", "nadia styles", "nautica thorn",
	"olivia del rio", "penny porsche", "raylene", "rebecca lord", "regan starr",
	"shayla laveaux", "shyla stylez", "sunny leone", "tawny roberts",
	"bambi woods", "gloria leonard", "annette haven", "tamara lee", "sharon mitchell",
	"samantha strong", "shayla laveaux", "shelbee myne", "sheri st clair", "sondra sommers",
	"suze randall", "sylvia kristel", "taija rae", "tammy parks", "tanya lawson",
	"tara aire", "tara gold", "tasha voux", "teri diver", "tiffany storm",
	"tom byron", "tracey adams", "trinity loren", "trudi", "victoria paris",
	"vivian parks", "viviana", "wade nichols", "yolanda", "zara whites",
	"alexandra quinn", "amber woods", "angela baron", "barbara dare", "brandy alexandre",
	"brittany o connell", "bunny bleu", "cara lott", "careena collins", "cassandra leigh",
	"chessie moore", "crystal wilder", "danielle rogers", "debi diamond", "deidre holland",
	"dyanna lauren", "ebony ayes", "elle rio", "erica boyer", "gail force",
	"janette littledove", "jennifer peace", "jeanna fine", "keisha", "kim chambers",
	"kitten natividad", "krista lane", "kristara barrington", "kylie ireland", "lacey rose",
	"lauryl canyon", "leanna heart", "letha weapons", "lois ayres", "lorrie lovett",
	"lynn lemay", "madison", "megan leigh", "mia powers", "micki lynn",
	"mikki finn", "missy warner", "moana pozzi", "monique demoan", "nina deponca",
}

// performersRecent - Recent/Emerging performers
var performersRecent = []string{
	"anna claire clouds", "blake blossom", "kenzie anne", "kylie rocket",
	"lulu chu", "maddy may", "madi collins", "minxx marley", "nala nova",
	"natalia nix", "nicole aria", "paisley porter", "quinn wilde", "reagan lush",
	"siri", "sky bri", "spencer bradley", "tommy king", "vanessa cage",
	"violet myers", "willow ryder", "xxlayna marie", "yumi sin", "zuzu sweet",
	"alyx star", "ana foxxx", "angelina diamanti", "aria banks", "aria lee",
	"athena faris", "autumn falls", "avery cristy", "bella rolland", "blake blossom",
	"brooklyn gray", "bunny colby", "chloe surreal", "delilah day", "eden ivy",
	"eliza ibarra", "ella knox", "emily willis", "emma hix", "gabbie carter",
	"gina valentina", "gianna dior", "harmony wonder", "hazel heart", "honey blossom",
	"indica flower", "jade venus", "jazlyn ray", "jewelz blu", "jia lissa",
	"jolee love", "kali roses", "kenna james", "kenzie reeves", "kiara cole",
	"kira noir", "kyler quinn", "lacy lennon", "lana rhoades", "lexi lore",
	"lily larimar", "lilly bell", "luna mills", "madi laine", "maitland ward",
	"maya bijou", "maya woulfe", "melody marks", "molly little", "mona azar",
	"natalie porkman", "natasha nice", "nia nacci", "nikki benz", "nina north",
	"octavia red", "paige owens", "pepper xo", "piper perri", "pristine edge",
	"quinn cummings", "rebel rhyder", "rhiannon ryder", "riley reid", "riley star",
	"sabina rouge", "savannah sixx", "scarlet skies", "sera ryder", "skylar vox",
	"sophia lux", "stella flex", "tiffany tatum", "vanna bardot", "vina sky",
	"gal ritchie", "chanel camryn", "freya parker", "leana lovings", "aften opal",
	"alexis tae", "laney grey", "catalina ossa", "kate dalia", "coco lovelock",
	"kayley gunner", "liz jordan", "mackenzie moss", "destiny cruz", "vanessa moon",
	"stevie moon", "serena hill", "sky pierce", "angel youngs", "diana grace",
	"emma rose", "vera king", "alex jones", "everly haze", "sloan harper",
	"jill taylor", "krissy knight", "anny aurora", "carolina sweets", "gwen stark",
	"kyra rose", "maya kendrick", "naomi swann", "piper madison", "quinn wilde",
	"scarlett fall", "theodora day", "whitney westgate", "zoe sparx", "aria kai",
	"chloe amour", "dixie lynn", "ember lace", "harmony rivers", "jana cova",
	"karissa kane", "luna bright", "megan sage", "nina elle", "paris white",
	"rose red", "sadie pop", "taylor blake", "una alexandar", "violet rae",
	"wren", "xandra sixx", "yara skye", "zara ryan", "addie andrews",
	"brianna beach", "cali carter", "dahlia sky", "emma butt", "gia grace",
}

// performersInternational - International performers from various countries
var performersInternational = []string{
	"amia miley", "aria alexander", "brenna sparks", "cecilia lion", "daisy stone",
	"eliza jane", "frida sante", "gina gerson", "heather night", "ivy lebelle",
	"jessa rhodes", "karlee grey", "lena anderson", "maria rya", "nancy ace",
	"olivia austin", "petra", "rebecca volpetti", "sasha foxxx", "tiffany watson",
	"uliana", "vera king", "wendy moon", "ximena lucero", "yarina",
	"zafira", "alexa tomas", "bettina dicapri", "carolina abril", "dolly diore",
	"eveline dellai", "flora fairy", "gina devine", "helena price", "irina bruni",
	"jenny blighe", "katya rodriguez", "lola bulgari", "mia melano", "nikita von james",
	"olya", "paula shy", "rosie skye", "silvia dellai",
	"taissia shanti", "ulyana", "victoria pure", "wanda nara", "xena",
	"yenna", "zena little", "apolonia lapiedra", "blanche bradburry", "cherry kiss",
	"diana grace", "elena koshka", "francesca le", "gigi allens", "henessy",
	"ivana sugar", "jasmine webb", "kira queen", "lilu moon", "mary kalisy",
	"nathaly cherie", "ornella morgan", "penelope cum", "rebecca sharon", "sybil",
	"taylee wood", "uma jolie", "veronica leal", "whitney wright", "xianna hill",
	"yarra", "zoe doll", "ana rose", "brianna beach", "cindy shine",
	"dee williams", "ella hughes", "gala brown", "heather vahn",
	"isabella soprano", "jada stevens", "katy rose", "lana seymour", "marilyn sugar",
	"natali ruby", "olivia nova", "peta jensen", "quinn cummings", "renata fox",
	"sienna west", "tina kay", "vanessa decker", "wiska",
	"lana roy", "agatha vega", "alina henessy", "allie haze", "alyssa branch",
	"anastasia brokelyn", "angie koks", "anna taylor", "ariana marie", "arteya",
	"casey calvert", "cherry jul", "cindy starfall", "connie carter", "dorothy black",
	"eva elfie", "ferrera gomez", "hanna lay", "ivana fukalot", "jenny fer",
	"julie skyhigh", "katarina hartlova", "katerina kay", "katya clover", "klaudia kelly",
	"kylie quinn", "leony aprill", "lilu moon", "linet slag", "lucia love",
	"lucy li", "madison parker", "mea melone", "melena maria rya", "mila milan",
	"mira sunset", "nancy a", "nelly kent", "nikita von james", "paulina soul",
	"raquel adan", "red fox", "roxy mendez", "sabrina deep", "sandra russo",
	"shione cooper", "silvia saint", "simony diamond", "susan ayn", "suzie carina",
	"sweet cat", "taisha", "tereza fox", "thalia mint", "thomas stone",
	"tiffany bright", "tina dove", "tyna shy", "valentina ross", "vicky vette",
	"victoria blaze", "victoria puppy", "victoria sweet", "winnie", "yasmine gold",
}

// performersAlt - Alternative/Punk/Goth/Tattoo performers
var performersAlt = []string{
	"joanna angel", "arabelle raphael", "dollie darko", "jessie lee", "sheena rose",
	"kleio valentien", "leigh raven", "nickole shea", "ophelia rain", "phoenix askani",
	"rizzo ford", "indica flower", "indigo augustine", "kenzie reeves", "raven bay",
	"scarlet de sade", "anna bell peaks", "bonnie rotten", "juelz ventura", "raven black",
	"daisy suxxx", "emma rose", "jessie saint", "lily lane", "monroe sweet",
	"nikita bellucci", "amber luke", "cece capella", "genevieve sinn", "lola fae",
	"octavia may", "phoenix madina", "rocky emerson", "sammie six", "sydney screams",
	"vera drake", "anuskatzz", "becky holt", "charlotte sartre", "draven star",
	"evilyn fierce", "georgia peach", "harlow harrison", "ivy wolfe", "jade piper",
	"katja kassin", "larkin love", "megan massacre", "napalm", "orion star",
	"proxy paige", "raven hart", "stella may", "tanya virago", "venus starr",
	"xena", "zombie dee", "aiden ashley", "belladonna", "cherry torn",
	"domino presley", "emily grey", "faye reagan", "goth charlotte", "harley quinn",
	"ivy poison", "jaclyn case", "larkin love", "megan inky",
	"nikita denise", "orchid", "pixie", "queen jezabel", "raven riley",
	"stoya", "tattoo sue", "una", "vera sky", "wednesday parker",
	"yuffie", "zelda", "anna rose", "blair witch",
	"carmen rivera", "delilah", "electra", "fae", "gore",
	"razor candi", "apnea", "kemper", "mosh", "razor",
	"riae", "sabien demonia", "scar 13", "splatterhead", "vampirella",
	"zombie girl", "anna deville", "black dahlia", "corvus", "decay",
	"evil angel", "flesh", "grimm", "hex", "inferna",
	"jinx", "karma", "luna dark", "medusa", "noir",
	"obsidian", "poison ivy", "queen of darkness", "raven", "siren",
	"thorn", "undead", "venom", "wicked", "xerces",
	"yew", "zeus", "abyss", "banshee", "crypt",
	"dusk", "eclipse", "feral", "ghost", "havoc",
	"ink", "jade", "kali", "lydia", "midnight",
	"nina dark", "onyx", "pandora", "quill", "razor",
	"sabine", "tempest", "ursa", "viper", "wraith",
	"xena dark", "yuki dark", "zephyr", "amber ivy", "betty curse",
	"cali suicide", "daizha morgann", "eden sher", "fay suicide", "gala",
}

// performersTransgender - Transgender performers
var performersTransgender = []string{
	"natalie mars", "chanel santini", "venus lux", "aubrey kate", "bailey jay",
	"domino presley", "jessy dubai", "kimber james", "korra del rio", "lena kelly",
	"mariana cordoba", "mia isabella", "sarina valentina", "shiri allwood",
	"sofia sanders", "ts madison", "vaniity", "victoria hyde", "wendy williams",
	"yasmin lee", "casey kisses", "daisy taylor", "ella hollywood", "eva maxim",
	"jade venus", "kayleigh coxx", "lena moon", "lianna lawson", "madison montag",
	"nikki vicious", "olivia love", "rebecca love", "river stark", "sara salazar",
	"tiffany starr", "victoria carvalho", "ariel demure", "carol penelope", "delia delions",
	"esmeralda brasil", "foxxy", "gaby souza", "hanna rios", "izzy wilde",
	"jessy lemos", "karla carrillo", "lina cavalli", "melyna merli", "nicole bahls",
	"pamela tays", "rafaella ferrari", "sabrina suzuki", "thaissa guedes", "valentina torres",
	"adriana rodrigues", "alessia valentino", "amy daly", "ana mancini", "annabelle lane",
	"ava doll", "brittney kade", "cayla sky", "chelsea poe", "dannii diesel",
	"destiny", "destiny porter", "doll sweet", "eden ivy", "emma rose",
	"erica cherry", "eva cassini", "gabrielle fox", "gianna rivera", "gisele bittencourt",
	"holly parker", "janelly ramirez", "jaquelin braxton", "jasmine jewels", "jelena vermilion",
	"jessica fox", "jonelle brooks", "josie wails", "juliana souza", "kali michaels",
	"kelli lox", "kendall dreams", "kim bella", "kylie maria", "lara tinelli",
	"leticia menezes", "lylith lavey", "mara nova", "natalie chen", "nina lawless",
	"penny tyler", "queenie sateen", "rosa velvet", "sasha de sade", "tiffany starr",
	"vanessa jhons", "wendy summers", "xena", "yasmin lee",
	"allysa etain", "avery jones", "bianca freire", "carmen moore", "crystal thayer",
	"daniela torres", "eva lin", "fernanda crystal", "gina hart", "holly sweet",
	"isabella sorrenti", "jane marie", "khloe hart", "lena lee", "morgan bailey",
	"natasha velez", "olivia starr", "paris pirelli", "raven roxx", "sasha strong",
	"tara emory", "valentina mia", "wendy williams", "ximena", "yolanda fox", "zara ryan",
	"alison dale", "brittany st jordan", "carla novaes", "daphne", "eva paradis",
	"freya wynn", "gianna michaels", "hazel tucker", "isa potter", "joanna jet",
	"karla lane", "lana tuls", "mandy mitchell", "nicole charming", "olivia kiss",
	"pearl sage", "raven dela croix", "sol", "tanya hyde", "uma jolie",
	"venus", "wendy summer", "xiomara", "yolanda", "zelda",
	"aspen brooks", "bianca hills", "carmen cruz", "domina", "eva lovia",
}

// performersBBW - BBW/Plus Size performers
var performersBBW = []string{
	"sofia rose", "mandy majestic", "samantha 38g", "cotton candi", "karla lane",
	"lady lynn", "mazzaratie monica", "nikky wilder", "sashaa juggs", "scarlett rouge",
	"eliza allure", "joslyn underwood", "lexxxi luxe", "ling ling", "maria moore",
	"sapphire rose", "velvet rose", "amazon darjeeling", "amiee roberts",
	"angelina castro", "apple bomb", "betty blac", "billie austin", "bunny de la cruz",
	"candy godiva", "cat bangles", "cc hollie", "cherry blossoms", "cocoa dawn",
	"cotton candy", "crystal clear", "curvy goddess", "deja", "destiny divine",
	"ebony goddess", "erin green", "felicia clover", "gidget", "glory foxxx",
	"harmony", "heavenly hailey", "jade rose", "jessie minx", "jitka",
	"juicy jackie", "karma may", "kendra secrets", "lady spice", "london andrews",
	"lovely louisa", "maserati", "michelle bond", "minnie mayhem", "nikki cars",
	"oak lynn", "peachez", "princess superstar", "queen rogue", "renee ross",
	"roxi red", "sadie spencer", "samantha g", "sarah rae", "scarlet lavey",
	"serenity sinn", "shanelle savage", "star staxx", "superstar", "tiffany blake",
	"treasure", "victoria cakes", "voluptuous val", "winter jade",
	"yvette bova", "zena star", "angie love", "betty boob", "chelsea charms",
	"desiree devine", "eliza allure", "felicia clover", "gigi", "holly jayde",
	"ivana baiul", "jenna june", "kelly shibari", "lexxxi luxe", "mia riley",
	"nikki santana", "olivia arden", "peyton thomas", "queen rogue", "roxy foxxx",
	"sienna hills", "teanna trump", "valgasmic", "winter jade", "yurizan beltran",
	"zelda morrison", "ava rose", "bella bendz", "claudia marie", "daphne rosen",
	"erin green", "fat bottom girl", "gidget", "harmony", "ivy black",
	"jenny hill", "kianna dior", "lucy lenore", "mona", "nikita",
	"olivia blake", "phat zane", "quickie", "rachel ray", "samantha star",
	"tiffany towers", "vanessa blake", "whitney stevens", "xena", "yolanda",
	"zariah", "alana lee", "busty dusty", "candy cotton", "deja",
	"ella knox", "felicity feline", "gigi allens", "hailey queen", "isis monroe",
	"jynx maze", "kimmie kaboom", "lana ivans", "micky bells", "natasha nice",
	"odyssey", "pinky", "quinn", "rosie raye", "scarlett johansson",
	"traci topps", "unique", "virgo", "wonderwoman", "xtina",
}

// performersAnal - Known for anal content
var performersAnal = []string{
	"adriana chechik", "proxy paige", "anna de ville", "alysa gap", "riley reid",
	"lena paul", "angela white", "jessa rhodes", "mia malkova", "valentina nappi",
	"eva lovia", "marley brinx", "abella danger", "jynx maze", "kelsi monroe",
	"jasmine jae", "bonnie rotten", "joanna angel", "katrina jade", "remy lacroix",
	"dana dearmond", "veronica avluv", "phoenix marie", "kristina rose", "allie haze",
	"asa akira", "sasha grey", "tory lane", "cathy heaven", "naomi woods",
	"casey calvert", "kendra lust", "sheena shaw", "amber rayne", "amy brooke",
	"flower tucci", "kristy black", "anissa kate", "melissa lauren",
	"aurora jolie", "carla crouz", "cindy crawford", "daisy marie", "elena koshka",
	"franceska jaimes", "gianna michaels", "heather harmon", "ida love", "jessica jaymes",
	"katie morgan", "lara de santis", "linda sweet", "madison parker", "maria ozawa",
	"nataly gold", "oliva la roche", "penny flame", "rachel roxxx", "samia duarte",
	"tiffany hopkins", "zoey monroe", "alexis ford", "brooklyn chase", "candy alexa",
	"devon lee", "eva angelina", "franceska le", "gina lynn", "haley wilde",
	"isis taylor", "kelly divine", "mandy muse",
	"nikita denise", "olivia olovely", "pinky", "rebeca linares", "shyla stylez",
	"angel wicky", "bibi noel", "claudia adams", "dahlia sky", "eden adams",
	"felony", "gina gerson", "holly berry", "india summer", "jodi west",
	"krissy lynn", "liza del sierra", "mia manarote", "nadia hilton", "olivia nice",
	"peta jensen", "queen bee", "remy lacroix", "sarah shevon", "tatiana kush",
	"vicky vette", "wendy moon", "xianna hill", "yuri luv", "zaya cassidy",
	"ashli orion", "bridgette b", "candy samira", "diana doll", "ella knox",
	"francesca le", "gina valentina", "harley jade", "isabella clark", "jenna jameson",
	"katrina jade", "lexi lore", "maddy oreilly", "natasha nice", "olga cabaeva",
	"phoenix askani", "raven bay", "serena torres", "tina dove", "una fairy",
	"vienna black", "wendy moon", "xianna hill", "yana", "zafira",
}

// performersOral - Known for oral/deepthroat
var performersOral = []string{
	"heather harmon", "sasha grey", "adriana chechik", "riley reid", "angela white",
	"lena paul", "abella danger", "kimmy granger", "mia malkova", "valentina nappi",
	"eva lovia", "jynx maze", "keisha grey", "marley brinx", "kelsi monroe",
	"jessa rhodes", "casey calvert", "remy lacroix", "kendra lust", "dana vespoli",
	"bonnie rotten", "joanna angel", "katrina jade", "charlotte sartre", "proxy paige",
	"anna de ville", "nina hartley", "rebecca lord", "vanessa del rio", "annette haven",
	"amber lynn", "crissy moran", "daisy marie", "felecia", "ginger lynn",
	"heather lee", "india summer", "janet mason", "kelly wells", "lisa ann",
	"missy martinez", "naomi russell", "olivia del rio", "penny flame", "rachel starr",
	"sativa rose", "tanya tate", "vanessa lane", "zoey holloway", "amy reid",
	"bella donna", "candice cardinele", "dani woodward", "evelyn lin", "francesca le",
	"gina ryder", "havana ginger", "isis love", "julia bond", "katja kassin",
	"lacy duvalle", "marie mccray", "nadia styles", "olivia winters", "priya rai",
	"rachel roxxx", "simone riley", "tiffany rayne", "victoria rae", "wendy divine",
	"gianna michaels", "emily willis", "gabbie carter", "autumn falls", "skylar vox",
	"blake blossom", "mia khalifa", "lana rhoades", "piper perri", "elsa jean",
	"tori black", "asa akira", "alexis texas", "madison ivy", "nicole aniston",
	"karlee grey", "kenzie reeves", "eliza ibarra", "vanna bardot", "maya woulfe",
	"lulu chu", "kylie rocket", "maddy may", "lily larimar", "emma starletto",
	"brooklyn gray", "athena faris", "spencer bradley", "rebel rhyder", "sky bri",
	"anna claire clouds", "kenzie anne", "paisley porter", "quinn wilde", "reagan lush",
	"alexis tae", "freya parker", "catalina ossa", "laney grey", "coco lovelock",
	"kayley gunner", "liz jordan", "mackenzie moss", "angel youngs", "diana grace",
}

// performersBrunette - Brunette performers
var performersBrunette = []string{
	"riley reid", "angela white", "lena paul", "abella danger", "mia malkova",
	"adriana chechik", "valentina nappi", "jessa rhodes", "keisha grey", "eva lovia",
	"jynx maze", "kelsi monroe", "casey calvert", "remy lacroix", "kendra lust",
	"dana vespoli", "bonnie rotten", "joanna angel", "katrina jade", "proxy paige",
	"anna de ville", "marley brinx", "kimmy granger", "veronica rodriguez", "mandy muse",
	"kenzie reeves", "karlee grey", "gianna dior", "vina sky", "jane wilde",
	"maya bijou", "gia derza", "emily willis", "gabbie carter", "autumn falls",
	"violet myers", "natasha nice", "brooklyn chase", "ava addams", "bridgette b",
	"ariella ferrera", "mercedes carrera", "rachel starr", "madison ivy", "asa akira",
	"priya rai", "jessica jaymes", "kianna dior", "hitomi tanaka", "cindy starfall",
	"london keyes", "marica hase", "katsuni", "alina li", "jade kush",
	"ayumi anime", "ember snow", "morgan lee", "polly pons", "may thai",
	"canela skin", "cassie del isla", "veronica leal", "esperanza gomez", "franceska jaimes",
	"kitty caprice", "rose monroe", "juliana vega", "serena santos", "luna star",
	"mia khalifa", "lana rhoades", "sasha grey", "jenna jameson", "tori black",
	"dani daniels", "christy mack", "lisa ann", "brandi love", "julia ann",
	"cory chase", "cherie deville", "india summer", "alexis fawx", "syren de mer",
	"dee williams", "reagan foxx", "london river", "kit mercer", "sheena ryder",
	"eliza ibarra", "maya woulfe", "vanna bardot", "brooklyn gray", "jazmin luv",
	"xxlayna marie", "olive glass", "sophia lux", "vienna rose", "emma starletto",
	"rebel rhyder", "avery cristy", "bunny colby", "elena koshka", "harmony rivers",
	"gianna michaels", "phoenix marie", "sara jay", "richelle ryan", "nikki benz",
	"jewels jade", "diamond foxxx", "ariella ferrera", "tanya tate", "jessica jaymes",
}

// performersBlonde - Blonde performers
var performersBlonde = []string{
	"elsa jean", "kendra sunderland", "samantha saint", "nicole aniston", "alexis texas",
	"kayden kross", "jesse jane", "stoya", "kagney linn karter",
	"bree olson", "tasha reign", "nikki benz", "shyla stylez", "aletta ocean",
	"capri cavanni", "anikka albrite", "lexi belle", "aj applegate", "carter cruise",
	"blair williams", "natalia starr", "jillian janson", "ashley fires",
	"india summer", "brandi love", "alexis fawx",
	"dee williams", "kit mercer", "london river", "ryan keely", "pristine edge",
	"katie morgan", "daisy stone", "layla london", "tori black",
	"piper perri", "emma hix", "kristen scott", "naomi woods",
	"haley reed", "lily rader", "aria lee", "chloe cherry",
	"lily larimar", "lilly bell", "melody marks", "harmony wonder",
	"blake blossom", "anna claire clouds", "sky bri", "vanna bardot",
	"dolly leigh", "vienna black", "scarlet skies", "vanessa cage", "spencer bradley",
	"maddy may", "kyler quinn", "lexi lore", "rebel rhyder", "athena faris",
	"mia malkova", "kendra lust", "julia ann", "cherie deville",
	"lena paul", "skylar vox", "gabbie carter", "kenzie reeves", "jessa rhodes",
	"leah gotti", "august ames", "peta jensen", "anissa kate",
	"jasmine jae", "tiffany doll", "anny aurora", "jenny wild", "victoria pure",
	"stella cox", "sienna day", "ella hughes", "tina kay", "liya silver",
	"nancy ace", "anna polina", "nathaly cherie", "sarah sultry", "katy rose",
	"alexis crystal", "henessy", "ivana sugar", "taissia shanti", "wendy moon",
	"gal ritchie", "chanel camryn", "freya parker", "leana lovings", "coco lovelock",
	"kayley gunner", "liz jordan", "mackenzie moss", "vanessa moon", "stevie moon",
	"serena hill", "sky pierce", "angel youngs", "emma rosie", "vera king",
	"sloan harper", "krissy knight", "gwen stark", "piper madison", "scarlett fall",
	"megan sage", "nina elle", "paris white", "rose red", "sadie pop",
	"taylor blake", "violet rae", "xandra sixx", "addie andrews", "dahlia sky",
}

// performersRedhead - Redhead performers
var performersRedhead = []string{
	"faye reagan", "marie mccray", "penny pax", "alex tanner", "dani jensen",
	"veronica vain", "ariel carmine", "bree daniels", "elle alexandra", "justine joli",
	"karlie montana", "krissy lynn", "lacy lennon", "madison ivy", "maitland ward",
	"maya kendrick", "melody jordan", "piper fawn", "rainia belle", "rose red",
	"siri", "stoya", "amarna miller", "amber dawn", "anny aurora",
	"arietta adams", "audrey hollander", "cameron love", "daisy stone", "dani daniels",
	"ella hughes", "emma snow", "ginger banks", "gwen stark", "harley quinn",
	"jayden cole", "jia lissa", "kendra james", "kierra wilde",
	"lauren phillips", "lexi belle", "maddy oreilly", "mia sollis", "natalie lust",
	"nikki rhodes", "ornella morgan", "red head queen", "ruby red",
	"sabrina sparx", "scarlett pain", "shana lane", "sophia locke", "sweet cherry",
	"taylor gunner", "trinity post", "veronica ricci", "victoria rae", "zara ryan",
	"abigail dupree", "alice green", "chloe carter", "fire red", "gwen adora",
	"ivy rose", "jessica robbin", "kira lake", "lilith lust", "miley may",
	"ashlyn molloy", "bree morgan", "carmen callaway", "dani woodward", "edyn blair",
	"farrah flower", "ginger patch", "heather carolin", "ivy jean", "janet mason",
	"kamryn jayde", "lucy fire", "marsha may", "natalie porkman", "olivia love",
	"pepper hart", "ramona cadence", "river lynn", "roxanne rae", "scarlett fever",
	"siri dahl", "teagan riley", "ursula d", "victoria marie", "velma voodoo",
	"wendy summers", "xana star", "yuki love", "zara white", "amber rouge",
	"bianca red", "cherry red", "debra wilson", "ember rayne", "flora red",
	"ginger hart", "holly red", "ivy hart", "jessie red", "kate red",
	"linda red", "melody james", "nikki red", "olivia redd", "piper red",
	"reba", "scarlett red", "tessa lane", "uma red", "vicky red",
	"willow hayes", "xena red", "yvette", "zoe voss", "anna lee",
	"bonnie kinz", "crystal red", "dawn avril", "ella fire", "flora",
}

// performersInterracial - Interracial specialists
var performersInterracial = []string{
	"riley reid", "angela white", "abella danger", "lena paul", "adriana chechik",
	"valentina nappi", "jynx maze", "kelsi monroe", "marley brinx", "keisha grey",
	"bonnie rotten", "katrina jade", "phoenix marie", "anna de ville", "proxy paige",
	"brooklyn chase", "kendra lust", "alexis fawx", "bridgette b", "ava addams",
	"nikki benz", "brandi love", "julia ann", "cherie deville", "india summer",
	"syren de mer", "lisa ann", "rachel starr", "madison ivy", "tori black",
	"asa akira", "london keyes", "marica hase", "cindy starfall", "kaylani lei",
	"dana vespoli", "veronica avluv", "dana dearmond", "sasha grey", "jenna jameson",
	"mia khalifa", "lana rhoades", "piper perri", "elsa jean", "riley star",
	"kira noir", "ana foxxx", "daya knight", "scarlit scandal", "september reign",
	"misty stone", "chanell heart", "jenna foxx", "diamond jackson", "anya ivy",
	"teanna trump", "moriah mills", "sarah banks", "nicole bexley", "harley dean",
	"jada fire", "naomi banxx", "lacey duvalle", "marie luv", "kapri styles",
	"skin diamond", "jezabel vessir", "osa lovely", "honey gold", "demi sutra",
	"lexington steele", "mandingo", "prince yahshua", "rico strong", "sean michaels",
	"dredd", "jax slayher", "isiah maxwell", "rob piper", "ricky johnson",
	"shane diesel", "flash brown", "jason brown", "jon jon", "jovan jordan",
	"mr marcus", "rico dawg", "nat turnher", "jason luv", "jovan jordan",
	"julio gomez", "louie smalls", "prince yahshua", "rob piper", "sean michaels",
	"shorty mac", "sly diggler", "slim poke", "wesley pipes", "byron long",
	"emily willis", "gabbie carter", "autumn falls", "blake blossom", "skylar vox",
	"violet myers", "kenzie reeves", "eliza ibarra", "maya woulfe", "gianna dior",
	"lulu chu", "kylie rocket", "maddy may", "lily larimar", "vanna bardot",
	"anna claire clouds", "kenzie anne", "spencer bradley", "sky bri", "chloe cherry",
}

// performersAmateur - Amateur/OnlyFans crossovers
var performersAmateur = []string{
	"mia khalifa", "lana rhoades", "riley reid", "bella thorne", "blac chyna",
	"bhad bhabie", "iggy azalea", "pia mia", "trisha paytas", "tana mongeau",
	"corinna kopf", "erica mena", "safaree samuels", "maitland ward", "farrah abraham",
	"tyga", "cardi b", "amber rose", "sky bri", "kazumi",
	"emily lynne", "serenity cox", "violet myers", "kendra sunderland", "autumn falls",
	"emily willis", "gabbie carter", "lena paul", "anna claire clouds", "blake blossom",
	"natalie reynolds", "alex adams", "eva elfie", "lexi lore", "angel youngs",
	"molly little", "coco lovelock", "mackenzie moss", "diana grace", "stevie moon",
	"sweetie fox", "zazie skymm", "eva maxim", "ellie eilish", "fiona frost",
	"harmony wonder", "hime marie", "indica flower", "jade venus", "jazlyn ray",
	"jewelz blu", "jia lissa", "kali roses", "kate quinn", "kayley gunner",
	"kimmy granger", "kyler quinn", "lacy lennon", "lily larimar", "lilly bell",
	"liz jordan", "luna star", "madi collins", "maya bijou", "melody marks",
	"mia split", "minxx marley", "nala nova", "natalie porkman", "natalia nix",
	"nicole aria", "octavia red", "olive glass", "paige owens", "paisley porter",
	"pepper xo", "piper perri", "quinn wilde", "reagan lush", "rebel rhyder",
	"rhiannon ryder", "sabina rouge", "savannah sixx", "scarlet skies", "sera ryder",
	"skylar vox", "sophia lux", "spencer bradley", "theodora day", "tiffany tatum",
	"tommy king", "vanessa cage", "vanna bardot", "vienna rose", "violet starr",
	"willow ryder", "xxlayna marie", "yumi sin", "zuzu sweet", "abella danger",
	"adriana chechik", "alexis fawx", "angela white", "asa akira", "kendra lust",
	"lisa ann", "madison ivy", "nicole aniston", "phoenix marie", "rachel starr",
	"sasha grey", "tori black", "brandi love", "julia ann", "cherie deville",
	"india summer", "brazzers", "reality kings", "naughty america", "bangbros",
}

// performersFitness - Fitness/Athletic performers
var performersFitness = []string{
	"kendra lust", "brandi love", "jewels jade", "lisa ann", "phoenix marie",
	"rachel starr", "nicole aniston", "alexis texas", "madison ivy", "tori black",
	"ava addams", "richelle ryan", "bridgette b", "ariella ferrera", "brooklyn chase",
	"ryan keely", "sheena ryder", "brittany andrews", "reagan foxx", "cherie deville",
	"cory chase", "london river", "syren de mer", "kit mercer", "pristine edge",
	"karma rx", "romi rain", "chanel preston", "alena croft", "lauren phillips",
	"angela white", "lena paul", "abella danger", "adriana chechik", "riley reid",
	"kelsi monroe", "jynx maze", "marley brinx", "bonnie rotten", "katrina jade",
	"dana vespoli", "veronica avluv", "dana dearmond", "india summer", "alexis fawx",
	"dee williams", "mona azar", "casca akashova", "penny archer", "lexi luna",
	"gabbie carter", "autumn falls", "blake blossom", "skylar vox", "violet myers",
	"natasha nice", "ella knox", "maitland ward", "stella cox", "gianna dior",
	"emily willis", "eliza ibarra", "maya woulfe", "vanna bardot", "lily larimar",
	"lilly bell", "anna claire clouds", "kenzie anne", "kylie rocket", "lulu chu",
	"maddy may", "spencer bradley", "sky bri", "kenzie reeves", "karlee grey",
	"nia nacci", "daya knight", "scarlit scandal", "ana foxxx", "kira noir",
	"misty stone", "diamond jackson", "moriah mills", "teanna trump", "harley dean",
	"esperanza gomez", "franceska jaimes", "luna star", "rose monroe", "serena santos",
	"marica hase", "cindy starfall", "london keyes", "jade kush", "ember snow",
	"aletta ocean", "jasmine jae", "tina kay", "stella cox", "sienna day",
}

// performersCosplay - Cosplay specialists
var performersCosplay = []string{
	"joanna angel", "arabelle raphael", "charlotte sartre", "dollie darko", "kleio valentien",
	"leigh raven", "indica flower", "anna bell peaks", "jynx maze", "abella danger",
	"riley reid", "emily willis", "gabbie carter", "autumn falls", "chloe cherry",
	"lacy lennon", "melody marks", "lilly bell", "blake blossom", "lily larimar",
	"emma starletto", "xxlayna marie", "maya woulfe", "eliza ibarra", "vanna bardot",
	"brooklyn gray", "jazmin luv", "olive glass", "vienna rose", "rebel rhyder",
	"bunny colby", "athena faris", "kenzie reeves", "karlee grey", "gianna dior",
	"lulu chu", "kylie rocket", "maddy may", "spencer bradley", "sky bri",
	"anna claire clouds", "kenzie anne", "paisley porter", "quinn wilde", "reagan lush",
	"rae lil black", "ayumi anime", "marica hase", "jade kush", "cindy starfall",
	"london keyes", "ember snow", "alina li", "katsuni", "asa akira",
	"hitomi tanaka", "yua mikami", "eimi fukada", "maria ozawa", "sora aoi",
	"anri okita", "tsubasa amami", "julia", "kirara asuka", "mion sonoda",
	"kianna dior", "tera patrick", "kaylani lei", "priya rai", "jessica bangkok",
	"miko lee", "charmane star", "tia tanaka", "mia li", "morgan lee",
	"bella rolland", "melody marks", "hazel moore", "lola fae", "gina valentina",
	"violet myers", "natasha nice", "skylar vox", "ella knox", "natalie mars",
	"chanel santini", "bailey jay", "aubrey kate", "venus lux", "domino presley",
	"daisy taylor", "ella hollywood", "casey kisses", "eva maxim", "jade venus",
	"eva elfie", "sweetie fox", "nancy a", "sybil", "jia lissa",
}

// performersVR - VR specialists
var performersVR = []string{
	"riley reid", "abella danger", "angela white", "lena paul", "adriana chechik",
	"emily willis", "gabbie carter", "autumn falls", "blake blossom", "skylar vox",
	"violet myers", "natasha nice", "ella knox", "gianna dior", "eliza ibarra",
	"maya woulfe", "vanna bardot", "lily larimar", "lilly bell", "kenzie reeves",
	"karlee grey", "nia nacci", "daya knight", "ana foxxx", "kira noir",
	"lulu chu", "kylie rocket", "maddy may", "spencer bradley", "sky bri",
	"anna claire clouds", "kenzie anne", "paisley porter", "quinn wilde", "reagan lush",
	"alexis tae", "freya parker", "catalina ossa", "laney grey", "coco lovelock",
	"kayley gunner", "liz jordan", "mackenzie moss", "angel youngs", "diana grace",
	"valentina nappi", "jynx maze", "kelsi monroe", "marley brinx", "keisha grey",
	"nicole aniston", "alexis texas", "madison ivy", "rachel starr", "phoenix marie",
	"kendra lust", "brandi love", "julia ann", "cherie deville", "india summer",
	"alexis fawx", "syren de mer", "lisa ann", "dee williams", "reagan foxx",
	"london river", "kit mercer", "brooklyn chase", "richelle ryan", "sheena ryder",
	"asa akira", "marica hase", "cindy starfall", "london keyes", "jade kush",
	"aletta ocean", "jasmine jae", "tina kay", "stella cox", "sienna day",
	"ella hughes", "liya silver", "nancy ace", "jenny wild", "victoria pure",
	"luna star", "rose monroe", "serena santos", "canela skin", "veronica leal",
	"esperanza gomez", "franceska jaimes", "katana kombat", "victoria june", "sophia leone",
	"teanna trump", "diamond jackson", "anya ivy", "scarlit scandal", "september reign",
}

// performersStudioBrazzers - Brazzers studio exclusives/regulars
var performersStudioBrazzers = []string{
	"keiran lee", "johnny sins", "xander corvus", "mick blue", "ramon nomar",
	"danny d", "jordi el nino polla", "small hands", "seth gamble", "tyler nixon",
	"madison ivy", "rachel starr", "chanel preston", "romi rain", "lela star",
	"misty stone", "ryan conner", "rebecca more", "richelle ryan", "tia cyrus",
	"cecilia lion", "lauren phillips", "alena croft", "leigh darby", "sarah banks",
	"jasmine webb", "karma rx", "bridgette b", "riley reid", "abella danger",
	"angela white", "lena paul", "adriana chechik", "emily willis", "gabbie carter",
	"autumn falls", "blake blossom", "skylar vox", "violet myers", "natasha nice",
	"ella knox", "gianna dior", "eliza ibarra", "maya woulfe", "vanna bardot",
	"lily larimar", "lilly bell", "kenzie reeves", "karlee grey", "nia nacci",
	"daya knight", "ana foxxx", "kira noir", "lulu chu", "kylie rocket",
	"maddy may", "spencer bradley", "sky bri", "anna claire clouds", "kenzie anne",
	"paisley porter", "quinn wilde", "reagan lush", "alexis tae", "freya parker",
	"catalina ossa", "laney grey", "coco lovelock", "kayley gunner", "liz jordan",
	"mackenzie moss", "angel youngs", "diana grace", "nicole aniston", "alexis texas",
	"phoenix marie", "kendra lust", "brandi love", "julia ann", "cherie deville",
	"india summer", "alexis fawx", "syren de mer", "lisa ann", "dee williams",
	"reagan foxx", "london river", "kit mercer", "brooklyn chase", "sheena ryder",
	"ava addams", "nikki benz", "jewels jade", "diamond foxxx", "ariella ferrera",
	"veronica avluv", "dana dearmond", "tanya tate", "jessica jaymes", "kianna dior",
}

// performersStudioRealityKings - Reality Kings studio exclusives/regulars
var performersStudioRealityKings = []string{
	"jmac", "brick danger", "tony rubino", "sean lawless", "kyle mason",
	"tyler steel", "bambino", "duncan saint", "oliver davis", "chad white",
	"alexis fawx", "bridgette b", "kendra lust", "richelle ryan", "ava addams",
	"ariella ferrera", "cherie deville", "india summer", "julia ann", "brandi love",
	"riley reid", "abella danger", "angela white", "lena paul", "gabbie carter",
	"emily willis", "autumn falls", "blake blossom", "skylar vox", "violet myers",
	"natasha nice", "ella knox", "gianna dior", "eliza ibarra", "maya woulfe",
	"luna star", "rose monroe", "serena santos", "canela skin", "veronica leal",
	"esperanza gomez", "juliana vega", "kitty caprice", "diamond kitty", "lela star",
	"teanna trump", "diamond jackson", "anya ivy", "moriah mills", "harley dean",
	"ana foxxx", "kira noir", "daya knight", "scarlit scandal", "september reign",
	"jenna foxx", "chanell heart", "misty stone", "layton benton", "sarah banks",
	"asa akira", "marica hase", "cindy starfall", "london keyes", "jade kush",
	"alina li", "katsuni", "ember snow", "morgan lee", "kalina ryu",
	"aletta ocean", "jasmine jae", "tina kay", "stella cox", "sienna day",
	"ella hughes", "liya silver", "nancy ace", "jenny wild", "victoria pure",
	"kelsi monroe", "jynx maze", "marley brinx", "valerie kay", "mandy muse",
	"rachel starr", "madison ivy", "nicole aniston", "alexis texas", "phoenix marie",
	"tori black", "kayden kross", "jesse jane", "remy lacroix", "dani daniels",
	"sasha grey", "jenna jameson", "gianna michaels", "sara jay", "kagney linn karter",
}

// performersStudioNaughtyAmerica - Naughty America studio regulars
var performersStudioNaughtyAmerica = []string{
	"johnny sins", "tyler nixon", "seth gamble", "ryan mclane", "lucas frost",
	"chad white", "damon dice", "codey steele", "van wylde", "nathan bronson",
	"brandi love", "julia ann", "cherie deville", "india summer", "alexis fawx",
	"syren de mer", "dee williams", "reagan foxx", "london river", "kit mercer",
	"kendra lust", "lisa ann", "brooklyn chase", "richelle ryan", "sheena ryder",
	"ryan keely", "pristine edge", "brittany andrews", "silvia saige", "rachael cavalli",
	"riley reid", "abella danger", "angela white", "lena paul", "adriana chechik",
	"emily willis", "gabbie carter", "autumn falls", "blake blossom", "skylar vox",
	"violet myers", "natasha nice", "ella knox", "gianna dior", "eliza ibarra",
	"maya woulfe", "vanna bardot", "lily larimar", "lilly bell", "kenzie reeves",
	"karlee grey", "nia nacci", "daya knight", "ana foxxx", "kira noir",
	"lulu chu", "kylie rocket", "maddy may", "spencer bradley", "sky bri",
	"anna claire clouds", "kenzie anne", "paisley porter", "quinn wilde", "reagan lush",
	"ava addams", "nikki benz", "bridgette b", "ariella ferrera", "jewels jade",
	"diamond foxxx", "mercedes carrera", "tanya tate", "jessica jaymes", "kianna dior",
	"luna star", "rose monroe", "esperanza gomez", "franceska jaimes", "canela skin",
	"marica hase", "cindy starfall", "london keyes", "jade kush", "ember snow",
	"aletta ocean", "jasmine jae", "tina kay", "stella cox", "sienna day",
	"teanna trump", "diamond jackson", "moriah mills", "harley dean", "sarah banks",
	"jenna foxx", "chanell heart", "misty stone", "scarlit scandal", "september reign",
}

// Performers is the combined list of all performers from all categories
var Performers = func() []string {
	// Pre-allocate with estimated capacity
	all := make([]string, 0, 8000)

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
	all = append(all, performersBBW...)
	all = append(all, performersAnal...)
	all = append(all, performersOral...)
	all = append(all, performersBrunette...)
	all = append(all, performersBlonde...)
	all = append(all, performersRedhead...)
	// New categories
	all = append(all, performersInterracial...)
	all = append(all, performersAmateur...)
	all = append(all, performersFitness...)
	all = append(all, performersCosplay...)
	all = append(all, performersVR...)
	all = append(all, performersStudioBrazzers...)
	all = append(all, performersStudioRealityKings...)
	all = append(all, performersStudioNaughtyAmerica...)

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
