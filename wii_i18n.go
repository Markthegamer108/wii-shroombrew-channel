package main

// Wii shop translations. The Wii system menu language selects the column.
// The Wii supports exactly ten languages; Portuguese is kept for the website.
// Missing or empty entries fall back to English.

import "strings"

// Column order of every row below.
var wiiLangs = []string{"en", "ja", "de", "fr", "es", "it", "nl", "zh", "zht", "ko", "pt"}

var wiiLangNames = map[string]string{
	"en": "English", "ja": "日本語", "de": "Deutsch", "fr": "Français", "es": "Español", "it": "Italiano",
	"nl": "Nederlands", "zh": "简体中文", "zht": "繁體中文", "ko": "한국어", "pt": "Português",
}

var wiiText = map[string]map[string]string{}

func row(key string, v ...string) {
	for i, lang := range wiiLangs {
		if i >= len(v) || v[i] == "" {
			continue
		}
		if wiiText[lang] == nil {
			wiiText[lang] = map[string]string{}
		}
		wiiText[lang][key] = v[i]
	}
}

// wiiLanguageFromSystem maps the language string the console reports
// (ECDeviceInfo.language) to one of our codes. Unknown values give "".
func wiiLanguageFromSystem(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	switch s {
	case "ja", "jp", "jpn", "japanese":
		return "ja"
	case "en", "eng", "english":
		return "en"
	case "de", "ger", "deu", "german":
		return "de"
	case "fr", "fre", "fra", "french":
		return "fr"
	case "es", "spa", "spanish":
		return "es"
	case "it", "ita", "italian":
		return "it"
	case "nl", "dut", "nld", "dutch":
		return "nl"
	case "zh", "zh-cn", "zh-hans", "chs", "zhs":
		return "zh"
	case "zh-tw", "zh-hant", "cht", "zht":
		return "zht"
	case "ko", "kor", "korean":
		return "ko"
	case "pt", "por":
		return "pt"
	}
	return ""
}

func init() {
	//   key          en | ja | de | fr | es | it | nl | zh | zht | ko | pt
	row("back", "Back", "戻る", "Zurück", "Retour", "Atrás", "Indietro", "Terug", "返回", "返回", "뒤로", "Voltar")
	row("home", "Shop Home", "ショップのトップ", "Shop-Start", "Accueil de la boutique", "Inicio de la tienda", "Home del negozio", "Shop-start", "商店首页", "商店首頁", "샵 홈", "Início da loja")
	row("ok", "OK", "OK", "OK", "OK", "Aceptar", "OK", "OK", "确定", "確定", "확인", "OK")
	row("download", "Download", "ダウンロード", "Herunterladen", "Télécharger", "Descargar", "Scarica", "Downloaden", "下载", "下載", "다운로드", "Baixar")
	row("add_points", "Add Wii Points", "Wiiポイントを追加", "Wii-Punkte hinzufügen", "Ajouter des Points Wii", "Añadir Puntos Wii", "Aggiungi Punti Wii", "Wii-punten toevoegen", "添加Wii点数", "新增Wii點數", "Wii 포인트 추가", "Adicionar Pontos Wii")
	row("continue", "Continue Shopping", "買い物を続ける", "Weiter einkaufen", "Continuer les achats", "Seguir comprando", "Continua lo shopping", "Verder winkelen", "继续购物", "繼續購物", "쇼핑 계속하기", "Continuar comprando")
	row("wii_points", "Wii Points", "Wiiポイント", "Wii-Punkte", "Points Wii", "Puntos Wii", "Punti Wii", "Wii-punten", "Wii点数", "Wii點數", "Wii 포인트", "Pontos Wii")
	row("sel_conf", "Selection Confirmation", "選択の確認", "Auswahl bestätigen", "Confirmation de la sélection", "Confirmar selección", "Conferma selezione", "Selectie bevestigen", "确认选择", "確認選擇", "선택 확인", "Confirmar seleção")
	row("need_more", "You need more Wii Points.", "Wiiポイントが足りません。", "Du brauchst mehr Wii-Punkte.", "Il vous faut plus de Points Wii.", "Necesitas más Puntos Wii.", "Servono più Punti Wii.", "Je hebt meer Wii-punten nodig.", "您需要更多Wii点数。", "您需要更多Wii點數。", "Wii 포인트가 더 필요합니다.", "Você precisa de mais Pontos Wii.")
	row("dl_this", "Download this software?", "このソフトをダウンロードしますか？", "Diese Software herunterladen?", "Télécharger ce logiciel ?", "¿Descargar este software?", "Scaricare questo software?", "Deze software downloaden?", "下载此软件吗？", "下載此軟體嗎？", "이 소프트웨어를 다운로드할까요?", "Baixar este software?")
	row("dl_msg", "Download this homebrew package for {n} Wii Points.", "このホームブリューを{n}Wiiポイントでダウンロードします。", "Dieses Homebrew-Paket für {n} Wii-Punkte herunterladen.", "Télécharger ce paquet homebrew pour {n} Points Wii.", "Descargar este paquete homebrew por {n} Puntos Wii.", "Scarica questo pacchetto homebrew per {n} Punti Wii.", "Dit homebrew-pakket downloaden voor {n} Wii-punten.", "以{n}Wii点数下载此自制软件包。", "以{n}Wii點數下載此自製軟體包。", "이 홈브류 패키지를 {n} Wii 포인트로 다운로드합니다.", "Baixar este pacote homebrew por {n} Pontos Wii.")
	row("connecting", "Connecting. Please wait.", "接続中です。お待ちください。", "Verbinde. Bitte warten.", "Connexion en cours. Veuillez patienter.", "Conectando. Espera, por favor.", "Connessione in corso. Attendi.", "Verbinden. Even geduld.", "正在连接，请稍候。", "正在連線，請稍候。", "연결 중입니다. 잠시 기다려 주세요.", "Conectando. Aguarde.")
	row("downloading", "Downloading software...", "ソフトをダウンロード中...", "Software wird heruntergeladen...", "Téléchargement du logiciel...", "Descargando software...", "Download del software...", "Software downloaden...", "正在下载软件...", "正在下載軟體...", "소프트웨어 다운로드 중...", "Baixando software...")
	row("verifying", "Verifying software...", "ソフトを確認中...", "Software wird geprüft...", "Vérification du logiciel...", "Verificando software...", "Verifica del software...", "Software controleren...", "正在验证软件...", "正在驗證軟體...", "소프트웨어 확인 중...", "Verificando software...")
	row("ready", "Your homebrew package is ready.", "ホームブリューの準備ができました。", "Dein Homebrew-Paket ist bereit.", "Votre paquet homebrew est prêt.", "Tu paquete homebrew está listo.", "Il tuo pacchetto homebrew è pronto.", "Je homebrew-pakket is klaar.", "您的自制软件包已准备好。", "您的自製軟體包已準備好。", "홈브류 패키지가 준비되었습니다.", "Seu pacote homebrew está pronto.")
	row("interrupted", "Connection interrupted. Return and try again.", "接続が中断されました。戻ってやり直してください。", "Verbindung unterbrochen. Zurückkehren und erneut versuchen.", "Connexion interrompue. Revenez et réessayez.", "Conexión interrumpida. Vuelve e inténtalo de nuevo.", "Connessione interrotta. Torna indietro e riprova.", "Verbinding verbroken. Ga terug en probeer opnieuw.", "连接已中断，请返回重试。", "連線已中斷，請返回重試。", "연결이 끊어졌습니다. 돌아가서 다시 시도하세요.", "Conexão interrompida. Volte e tente novamente.")
	row("unavailable", "Download unavailable. No points were charged.", "ダウンロードできません。ポイントは使われていません。", "Download nicht verfügbar. Es wurden keine Punkte abgebucht.", "Téléchargement indisponible. Aucun point débité.", "Descarga no disponible. No se cobraron puntos.", "Download non disponibile. Nessun punto addebitato.", "Download niet beschikbaar. Er zijn geen punten afgeschreven.", "无法下载，未扣除点数。", "無法下載，未扣除點數。", "다운로드할 수 없습니다. 포인트는 차감되지 않았습니다.", "Download indisponível. Nenhum ponto foi cobrado.")

	// Wii Points page and card entry
	row("pts_sub", "Shopping Made Easy with Wii Points", "Wiiポイントで楽々お買い物", "Einfach einkaufen mit Wii-Punkten", "Faites vos achats facilement avec les Points Wii", "Compras fáciles con los Puntos Wii", "Fare acquisti è facile con i Punti Wii", "Gemakkelijk winkelen met Wii-punten", "用Wii点数轻松购物", "用Wii點數輕鬆購物", "Wii 포인트로 쉽게 쇼핑하세요", "Compre com facilidade com Pontos Wii")
	row("pts_text", "Go to {site} to get your Wii Points card code!", "{site} にアクセスして、Wiiポイントカードのコードを手に入れよう！", "Gehe zu {site}, um deinen Wii-Punkte-Karten-Code zu erhalten!", "Rendez-vous sur {site} pour obtenir votre code de carte Points Wii !", "¡Ve a {site} para conseguir el código de tu tarjeta de Puntos Wii!", "Vai su {site} per ottenere il codice della tua carta Punti Wii!", "Ga naar {site} voor je Wii-puntenkaartcode!", "前往 {site} 获取您的Wii点数卡代码！", "前往 {site} 取得您的Wii點數卡代碼！", "{site}에서 Wii 포인트 카드 코드를 받으세요!", "Acesse {site} para obter o código do seu cartão de Pontos Wii!")
	row("pts_text2", "Each code is worth 1000 Wii Points and can be used once.", "1つのコードは1000Wiiポイント分で、1回だけ使えます。", "Jeder Code ist 1000 Wii-Punkte wert und nur einmal verwendbar.", "Chaque code vaut 1000 Points Wii et n'est utilisable qu'une fois.", "Cada código vale 1000 Puntos Wii y se puede usar una vez.", "Ogni codice vale 1000 Punti Wii e si usa una sola volta.", "Elke code is 1000 Wii-punten waard en kan één keer gebruikt worden.", "每个代码价值1000 Wii点数，仅限使用一次。", "每個代碼價值1000 Wii點數，僅限使用一次。", "코드 하나는 1000 Wii 포인트이며 한 번만 사용할 수 있습니다.", "Cada código vale 1000 Pontos Wii e pode ser usado uma vez.")
	row("pts_btn", "Enter Card Code", "カードコードを入力", "Kartencode eingeben", "Saisir le code de la carte", "Introducir código", "Inserisci codice carta", "Kaartcode invoeren", "输入卡代码", "輸入卡代碼", "카드 코드 입력", "Inserir código do cartão")
	row("card_title", "Wii Points Card", "Wiiポイントカード", "Wii-Punkte-Karte", "Carte Points Wii", "Tarjeta de Puntos Wii", "Carta Punti Wii", "Wii-puntenkaart", "Wii点数卡", "Wii點數卡", "Wii 포인트 카드", "Cartão de Pontos Wii")
	row("card_prompt", "Enter your 16-character card code.", "16文字のカードコードを入力してください。", "Gib deinen 16-stelligen Kartencode ein.", "Saisissez votre code de carte à 16 caractères.", "Introduce el código de 16 caracteres de tu tarjeta.", "Inserisci il codice della carta di 16 caratteri.", "Voer je kaartcode van 16 tekens in.", "请输入16位卡代码。", "請輸入16位卡代碼。", "16자리 카드 코드를 입력하세요.", "Digite o código de 16 caracteres do seu cartão.")
	row("card_btn", "Redeem", "引き換える", "Einlösen", "Valider", "Canjear", "Riscatta", "Inwisselen", "兑换", "兌換", "사용하기", "Resgatar")
	row("card_ok", "{n} Wii Points added!", "{n}Wiiポイントが追加されました！", "{n} Wii-Punkte hinzugefügt!", "{n} Points Wii ajoutés !", "¡{n} Puntos Wii añadidos!", "{n} Punti Wii aggiunti!", "{n} Wii-punten toegevoegd!", "已添加{n}Wii点数！", "已新增{n}Wii點數！", "Wii 포인트 {n}점이 추가되었습니다!", "{n} Pontos Wii adicionados!")
	row("card_bad", "That code is not valid.", "このコードは無効です。", "Dieser Code ist ungültig.", "Ce code n'est pas valide.", "Ese código no es válido.", "Questo codice non è valido.", "Deze code is ongeldig.", "此代码无效。", "此代碼無效。", "유효하지 않은 코드입니다.", "Esse código não é válido.")
	row("card_used", "That code was already used.", "このコードは使用済みです。", "Dieser Code wurde bereits verwendet.", "Ce code a déjà été utilisé.", "Ese código ya se usó.", "Questo codice è già stato usato.", "Deze code is al gebruikt.", "此代码已被使用。", "此代碼已被使用。", "이미 사용된 코드입니다.", "Esse código já foi usado.")
	row("card_expired", "That code has expired.", "このコードは期限切れです。", "Dieser Code ist abgelaufen.", "Ce code a expiré.", "Ese código ha caducado.", "Questo codice è scaduto.", "Deze code is verlopen.", "此代码已过期。", "此代碼已過期。", "만료된 코드입니다.", "Esse código expirou.")
	row("card_wait", "Too many tries. Wait {s} seconds.", "試行回数が多すぎます。{s}秒お待ちください。", "Zu viele Versuche. Warte {s} Sekunden.", "Trop d'essais. Attendez {s} secondes.", "Demasiados intentos. Espera {s} segundos.", "Troppi tentativi. Attendi {s} secondi.", "Te vaak geprobeerd. Wacht {s} seconden.", "尝试次数过多，请等待{s}秒。", "嘗試次數過多，請等待{s}秒。", "시도 횟수가 너무 많습니다. {s}초 기다려 주세요.", "Muitas tentativas. Aguarde {s} segundos.")
	row("card_err", "Something went wrong. Try again.", "問題が発生しました。もう一度お試しください。", "Etwas ist schiefgelaufen. Versuche es erneut.", "Un problème est survenu. Réessayez.", "Algo salió mal. Inténtalo de nuevo.", "Qualcosa è andato storto. Riprova.", "Er ging iets mis. Probeer het opnieuw.", "出错了，请重试。", "發生錯誤，請重試。", "문제가 발생했습니다. 다시 시도하세요.", "Algo deu errado. Tente novamente.")

	// Card entry page (P_02) - Wii Shop Channel original text
	row("c_r_title", "Wii Points Card", "Wiiポイントカード", "Wii-Punkte-Karte", "Carte Points Wii", "Tarjeta de Puntos Wii", "Carta Punti Wii", "Wii-puntenkaart", "Wii点数卡", "Wii點數卡", "Wii 포인트 카드", "Cartão de Pontos Wii")
	row("c_r_text", "Scratch off the silver area on the back of your Wii Points Card to reveal the 16-digit activation number.", "Wiiポイントカード裏面の銀色の部分を削って、16桁のアクティベーション番号を出してください。", "Kratzen Sie den silbernen Bereich auf der Rückseite Ihrer Wii-Punkte-Karte frei, um die 16-stellige Aktivierungsnummer zu erhalten.", "Gratter la zone argentée au dos de votre carte Points Wii pour faire apparaître le numéro d'activation à 16 chiffres.", "Rasca la zona plateada del reverso de tu tarjeta de Puntos Wii para revelar el número de activación de 16 dígitos.", "Grattare l'area argentata sul retro della carta Punti Wii per scoprire il numero di attivazione a 16 cifre.", "Krabbel het zilvergedeelte aan de achterkant van je Wii-puntenkaart open om het 16-cijferige activeringsnummer te zien.", "刮开Wii点数卡背面的银色区域以显示16位激活号码。", "刮開Wii點數卡背面的銀色區域以顯示16位啟用號碼。", "Wii 포인트 카드 뒷면의 은색 부분을 긁어 16자리 활성화 번호를 확인하세요.", "Raspe a área prateada no verso do seu Cartão de Pontos Wii para revelar o número de ativação de 16 dígitos.")
	row("c_r_label", "Activation Number", "アクティベーション番号", "Aktivierungsnummer", "Numéro d'activation", "Número de activación", "Numero di attivazione", "Activeringsnummer", "激活号码", "啟用號碼", "활성화 번호", "Número de ativação")
	row("c_go", "Redeem", "引き換える", "Einlösen", "Valider", "Canjear", "Riscatta", "Inwisselen", "兑换", "兌換", "사용하기", "Resgatar")

	// Error messages for card entry - original Wii Shop Channel text with <BR> for line breaks
	row("er_card_bad", "The activation number you entered is incorrect.<BR><BR>Please try again.", "入力されたアクティベーション番号が間違っています。<BR><BR>もう一度お試しください。", "Die eingegebene Aktivierungsnummer ist falsch.<BR><BR>Bitte versuchen Sie es erneut.", "Le numéro d'activation saisi est incorrect.<BR><BR>Veuillez réessayer.", "El número de activación que has introducido es incorrecto.<BR><BR>Inténtalo de nuevo.", "Il numero di attivazione che hai inserito non è corretto.<BR><BR>Riprova.", "Het ingevoerde activeringsnummer is onjuist.<BR><BR>Probeer het opnieuw.", "您输入的激活号码不正确。<BR><BR>请重试。", "您輸入的啟用號碼不正確。<BR><BR>請重試。", "입력하신 활성화 번호가 올바르지 않습니다.<BR><BR>다시 시도해 주세요.", "O número de ativação que você inseriu está incorreto.<BR><BR>Tente novamente.")
	row("er_card_cap", "Your balance is full.", "残高が上限に達しています。", "Ihr Guthaben ist voll.", "Votre solde est plein.", "Tu saldo está lleno.", "Il tuo saldo è pieno.", "Je saldo is vol.", "您的余额已满。", "您的餘額已滿。", "잔액이 가득 찼습니다.", "Seu saldo está cheio.")
	row("er_locked", "Too many wrong tries. Try again later.", "試行回数が多すぎます。後でもう一度お試しください。", "Zu viele falsche Versuche. Versuchen Sie es später erneut.", "Trop d'essais incorrects. Réessayez plus tard.", "Demasiados intentos fallidos. Inténtalo más tarde.", "Troppi tentativi errati. Riprova più tardi.", "Te veel foute pogingen. Probeer het later opnieuw.", "尝试次数过多，请稍后再试。", "嘗試次數過多，請稍後再試。", "시도 횟수가 너무 많습니다. 나중에 다시 시도해 주세요.", "Muitas tentativas incorretas. Tente novamente mais tarde.")

	// Points landing page (P_01)
	row("c_title", "Add Wii Points", "Wiiポイントを追加", "Wii-Punkte hinzufügen", "Ajouter des Points Wii", "Añadir Puntos Wii", "Aggiungi Punti Wii", "Wii-punten toevoegen", "添加Wii点数", "新增Wii點數", "Wii 포인트 추가", "Adicionar Pontos Wii")
	row("c_sub", "Add Wii Points to your account.", "Wiiポイントをアカウントに追加します。", "Fügen Sie Ihrem Konto Wii-Punkte hinzu.", "Ajoutez des Points Wii à votre compte.", "Añade Puntos Wii a tu cuenta.", "Aggiungi Punti Wii al tuo account.", "Voeg Wii-punten aan je account toe.", "向账户添加Wii点数。", "向帳戶新增Wii點數。", "계정에 Wii 포인트를 추가하세요.", "Adicione Pontos Wii à sua conta.")
	row("c_text", "Go to {site} to get your Wii Points card code!", "{site} にアクセスして、Wiiポイントカードのコードを手に入れよう！", "Gehe zu {site}, um deinen Wii-Punkte-Karten-Code zu erhalten!", "Rendez-vous sur {site} pour obtenir votre code de carte Points Wii !", "¡Ve a {site} para conseguir el código de tu tarjeta de Puntos Wii!", "Vai su {site} per ottenere il codice della tua carta Punti Wii!", "Ga naar {site} voor je Wii-puntenkaartcode!", "前往 {site} 获取您的Wii点数卡代码！", "前往 {site} 取得您的Wii點數卡代碼！", "{site}에서 Wii 포인트 카드 코드를 받으세요!", "Acesse {site} para obter o código do seu cartão de Pontos Wii!")
	row("c_text2", "Each code is worth 1000 Wii Points and can be used once.", "1つのコードは1000Wiiポイント分で、1回だけ使えます。", "Jeder Code ist 1000 Wii-Punkte wert und nur einmal verwendbar.", "Chaque code vaut 1000 Points Wii et n'est utilisable qu'une fois.", "Cada código vale 1000 Puntos Wii y se puede usar una vez.", "Ogni codice vale 1000 Punti Wii e si usa una sola volta.", "Elke code is 1000 Wii-punten waard en kan één keer gebruikt worden.", "每个代码价值1000 Wii点数，仅限使用一次。", "每個代碼價值1000 Wii點數，僅限使用一次。", "코드 하나는 1000 Wii 포인트이며 한 번만 사용할 수 있습니다.", "Cada código vale 1000 Pontos Wii e pode ser usado uma vez.")
	row("c_redeem_a", "Enter your 16-character", "16文字の", "16-stelligen", "votre code à 16", "tu código de 16", "il tuo codice a 16", "jouw 16-cijferige", "您的16位", "您的16位", "16자리", "seu código de 16")
	row("c_redeem_b", "card code here.", "カードコードを入力", "Kartencode hier ein", "caractères ici", "caracteres aquí", "caratteri qui", "kaartcode hier", "卡代码", "卡代碼", "카드 코드 입력", "caracteres aqui")
	row("c_continue_a", "Or buy Wii Points", "またはWiiポイントを購入", "Oder Wii-Punkte kaufen", "Ou acheter des Points Wii", "O comprar Puntos Wii", "O comprare Punti Wii", "Of Wii-punten kopen", "或购买Wii点数", "或購買Wii點數", "또는 Wii 포인트 구매", "Ou comprar Pontos Wii")
	row("c_continue_b", "with a credit card.", "クレジットカードで", "mit Kreditkarte", "par carte de crédit", "con tarjeta de crédito", "con carta di credito", "met creditcard", "使用信用卡", "使用信用卡", "신용카드로", "com cartão de crédito")

	// Buy points page (P_06)
	row("c_buy_title", "Add Wii Points", "Wiiポイントを追加", "Wii-Punkte hinzufügen", "Ajouter des Points Wii", "Añadir Puntos Wii", "Aggiungi Punti Wii", "Wii-punten toevoegen", "添加Wii点数", "新增Wii點數", "Wii 포인트 추가", "Adicionar Pontos Wii")
	row("c_buy_text", "Credit card purchases are not available.", "クレジットカードでの購入はできません。", "Kreditkartenzahlungen sind nicht verfügbar.", "Les achats par carte de crédit ne sont pas disponibles.", "No se pueden realizar compras con tarjeta de crédito.", "Gli acquisti con carta di credito non sono disponibili.", "Aankopen met creditcard zijn niet beschikbaar.", "无法使用信用卡购买。", "無法使用信用卡購買。", "신용카드 구매를 할 수 없습니다.", "Compras com cartão de crédito não estão disponíveis.")
	row("c_buy_text2", "Get Wii Points with a card code from the Shroombrew website.", "Shroombrewのサイトでカードコードを入手して、Wiiポイントを追加してください。", "Hol dir Wii-Punkte mit einem Kartencode von der Shroombrew-Website.", "Obtenez des Points Wii avec un code de carte du site Shroombrew.", "Consigue Puntos Wii con un código de tarjeta del sitio de Shroombrew.", "Ottieni Punti Wii con un codice carta dal sito Shroombrew.", "Krijg Wii-punten met een kaartcode van de Shroombrew-website.", "使用来自Shroombrew网站的卡代码获取Wii点数。", "使用來自Shroombrew網站的卡代碼取得Wii點數。", "Shroombrew 웹사이트의 카드 코드로 Wii 포인트를 받으세요.", "Obtenha Pontos Wii com um código de cartão do site do Shroombrew.")

	// Friends & rewards
	row("title", "Friends & Rewards", "フレンドと特典", "Freunde & Belohnungen", "Amis et récompenses", "Amigos y recompensas", "Amici e premi", "Vrienden & beloningen", "好友与奖励", "好友與獎勵", "친구와 보상", "Amigos e recompensas")
	row("code", "Your friend code", "あなたのフレンドコード", "Dein Freundescode", "Votre code ami", "Tu código de amigo", "Il tuo codice amico", "Jouw vriendcode", "您的好友代码", "您的好友代碼", "내 친구 코드", "Seu código de amigo")
	row("share", "Share it. When a friend enters it, you both get points.", "友だちに教えよう。入力するとふたりにポイントが届きます。", "Teile ihn. Wenn ein Freund ihn eingibt, bekommt ihr beide Punkte.", "Partagez-le. Quand un ami l'entre, vous gagnez tous les deux des points.", "Compártelo. Cuando un amigo lo introduzca, ambos recibís puntos.", "Condividilo. Quando un amico lo inserisce, ricevete entrambi dei punti.", "Deel hem. Als een vriend hem invoert, krijgen jullie allebei punten.", "分享给好友，好友输入后你们都能获得点数。", "分享給好友，好友輸入後你們都能獲得點數。", "친구에게 알려 주세요. 입력하면 둘 다 포인트를 받습니다.", "Compartilhe. Quando um amigo o digitar, vocês dois ganham pontos.")
	row("tier", "Reward level", "特典レベル", "Belohnungsstufe", "Niveau de récompense", "Nivel de recompensa", "Livello premio", "Beloningsniveau", "奖励等级", "獎勵等級", "보상 등급", "Nível de recompensa")
	row("none", "None yet", "まだありません", "Noch keine", "Aucun pour l'instant", "Ninguno aún", "Nessuno per ora", "Nog geen", "暂无", "暫無", "아직 없음", "Nenhum ainda")
	row("enter", "Enter a friend's code", "友だちのコードを入力", "Code eines Freundes eingeben", "Entrez le code d'un ami", "Introduce el código de un amigo", "Inserisci il codice di un amico", "Voer de code van een vriend in", "输入好友的代码", "輸入好友的代碼", "친구의 코드 입력", "Digite o código de um amigo")
	row("redeem", "Redeem", "引き換える", "Einlösen", "Valider", "Canjear", "Riscatta", "Inwisselen", "兑换", "兌換", "사용하기", "Resgatar")
	row("done", "Code accepted!", "コードを受け付けました！", "Code akzeptiert!", "Code accepté !", "¡Código aceptado!", "Codice accettato!", "Code geaccepteerd!", "代码已接受！", "代碼已接受！", "코드가 적용되었습니다!", "Código aceito!")
	row("gifts", "Gifts for you", "あなたへのプレゼント", "Geschenke für dich", "Cadeaux pour vous", "Regalos para ti", "Regali per te", "Cadeaus voor jou", "给您的礼物", "給您的禮物", "받은 선물", "Presentes para você")
	row("accept", "Accept", "受け取る", "Annehmen", "Accepter", "Aceptar", "Accetta", "Accepteren", "接受", "接受", "받기", "Aceitar")
	row("decline", "Decline", "断る", "Ablehnen", "Refuser", "Rechazar", "Rifiuta", "Weigeren", "拒绝", "拒絕", "거절", "Recusar")
	row("nogifts", "No gifts waiting.", "届いているプレゼントはありません。", "Keine Geschenke vorhanden.", "Aucun cadeau en attente.", "No hay regalos pendientes.", "Nessun regalo in attesa.", "Geen cadeaus in afwachting.", "没有待处理的礼物。", "沒有待處理的禮物。", "대기 중인 선물이 없습니다.", "Nenhum presente pendente.")
	row("updates", "Updates available", "アップデートあり", "Updates verfügbar", "Mises à jour disponibles", "Actualizaciones disponibles", "Aggiornamenti disponibili", "Updates beschikbaar", "有可用更新", "有可用更新", "업데이트 있음", "Atualizações disponíveis")
	row("noupdates", "Everything is up to date.", "すべて最新です。", "Alles ist aktuell.", "Tout est à jour.", "Todo está actualizado.", "Tutto è aggiornato.", "Alles is up-to-date.", "全部已是最新。", "全部已是最新。", "모두 최신 상태입니다.", "Tudo está atualizado.")
	row("from", "from", "送り主:", "von", "de", "de", "da", "van", "来自", "來自", "보낸 사람:", "de")
	row("remote_title", "Your remote code", "あなたのリモートコード", "Dein Fernzugriffs-Code", "Votre code à distance", "Tu código remoto", "Il tuo codice remoto", "Jouw afstandscode", "您的远程代码", "您的遠端代碼", "내 원격 코드", "Seu código remoto")
	row("remote_help", "Keep it secret. On {site} enter it under Friends to manage this Wii from your computer.", "他の人には教えないでください。{site} のフレンドで入力すると、パソコンからこのWiiを管理できます。", "Gib ihn nicht weiter. Auf {site} unter Freunde eingeben, um diese Wii vom Computer aus zu verwalten.", "Gardez-le secret. Sur {site}, saisissez-le dans Amis pour gérer cette Wii depuis votre ordinateur.", "Mantenlo en secreto. En {site}, introdúcelo en Amigos para gestionar esta Wii desde tu ordenador.", "Tienilo segreto. Su {site} inseriscilo in Amici per gestire questa Wii dal computer.", "Houd hem geheim. Voer hem op {site} in bij Vrienden om deze Wii vanaf je computer te beheren.", "请保密。在 {site} 的“好友”中输入，即可在电脑上管理这台Wii。", "請保密。在 {site} 的「好友」中輸入，即可在電腦上管理這台Wii。", "비밀로 유지하세요. {site}의 친구 메뉴에 입력하면 컴퓨터에서 이 Wii를 관리할 수 있습니다.", "Mantenha em segredo. Em {site}, digite em Amigos para gerenciar este Wii pelo computador.")
	row("recover", "Restore a Wii with its remote code", "リモートコードでWiiを復元", "Wii mit dem Fernzugriffs-Code wiederherstellen", "Restaurer une Wii avec son code à distance", "Restaurar una Wii con su código remoto", "Ripristina una Wii con il codice remoto", "Wii herstellen met de afstandscode", "用远程代码恢复Wii", "用遠端代碼還原Wii", "원격 코드로 Wii 복원", "Restaurar um Wii com o código remoto")
	row("recovered", "Wii restored!", "Wiiを復元しました！", "Wii wiederhergestellt!", "Wii restaurée !", "¡Wii restaurada!", "Wii ripristinata!", "Wii hersteld!", "Wii已恢复！", "Wii已還原！", "Wii가 복원되었습니다!", "Wii restaurado!")

	// user-facing errors
	row("e_badcode", "That friend code does not exist.", "そのフレンドコードは存在しません。", "Diesen Freundescode gibt es nicht.", "Ce code ami n'existe pas.", "Ese código de amigo no existe.", "Questo codice amico non esiste.", "Die vriendcode bestaat niet.", "该好友代码不存在。", "該好友代碼不存在。", "존재하지 않는 친구 코드입니다.", "Esse código de amigo não existe.")
	row("e_self", "You cannot use your own code.", "自分のコードは使えません。", "Du kannst deinen eigenen Code nicht verwenden.", "Vous ne pouvez pas utiliser votre propre code.", "No puedes usar tu propio código.", "Non puoi usare il tuo codice.", "Je kunt je eigen code niet gebruiken.", "不能使用自己的代码。", "不能使用自己的代碼。", "자신의 코드는 사용할 수 없습니다.", "Você não pode usar seu próprio código.")
	row("already", "You already entered a friend code.", "すでにフレンドコードを入力済みです。", "Du hast bereits einen Freundescode eingegeben.", "Vous avez déjà entré un code ami.", "Ya introdujiste un código de amigo.", "Hai già inserito un codice amico.", "Je hebt al een vriendcode ingevoerd.", "您已输入过好友代码。", "您已輸入過好友代碼。", "이미 친구 코드를 입력했습니다.", "Você já digitou um código de amigo.")
	row("e_full", "That code has reached its reward limit.", "このコードは特典の上限に達しました。", "Dieser Code hat sein Belohnungslimit erreicht.", "Ce code a atteint sa limite de récompenses.", "Ese código alcanzó su límite de recompensas.", "Questo codice ha raggiunto il limite di premi.", "Die code heeft zijn beloningslimiet bereikt.", "该代码已达到奖励上限。", "該代碼已達到獎勵上限。", "이 코드는 보상 한도에 도달했습니다.", "Esse código atingiu o limite de recompensas.")
	row("e_gift_gone", "That gift is no longer available.", "そのプレゼントはもう受け取れません。", "Dieses Geschenk ist nicht mehr verfügbar.", "Ce cadeau n'est plus disponible.", "Ese regalo ya no está disponible.", "Quel regalo non è più disponibile.", "Dat cadeau is niet meer beschikbaar.", "该礼物已不可用。", "該禮物已不可用。", "해당 선물은 더 이상 받을 수 없습니다.", "Esse presente não está mais disponível.")
	row("e_generic", "Something went wrong. Try again.", "問題が発生しました。もう一度お試しください。", "Etwas ist schiefgelaufen. Versuche es erneut.", "Un problème est survenu. Réessayez.", "Algo salió mal. Inténtalo de nuevo.", "Qualcosa è andato storto. Riprova.", "Er ging iets mis. Probeer het opnieuw.", "出错了，请重试。", "發生錯誤，請重試。", "문제가 발생했습니다. 다시 시도하세요.", "Algo deu errado. Tente novamente.")

	// Settings, help, news, activity
	row("settings", "Settings and Features", "設定と機能", "Einstellungen und Funktionen", "Paramètres et fonctions", "Ajustes y funciones", "Impostazioni e funzioni", "Instellingen en functies", "设置与功能", "設定與功能", "설정 및 기능", "Configurações e recursos")
	row("music", "Shop music:", "ショップのBGM:", "Shop-Musik:", "Musique de la boutique :", "Música de la tienda:", "Musica del negozio:", "Winkelmuziek:", "商店音乐：", "商店音樂：", "샵 음악:", "Música da loja:")
	row("music_on", "Music On", "BGMオン", "Musik an", "Musique activée", "Música sí", "Musica sì", "Muziek aan", "开启音乐", "開啟音樂", "음악 켜기", "Música ligada")
	row("music_off", "Music Off", "BGMオフ", "Musik aus", "Musique coupée", "Música no", "Musica no", "Muziek uit", "关闭音乐", "關閉音樂", "음악 끄기", "Música desligada")
	row("wallet_note", "Your points, gifts and purchases are saved on the server and kept for this Wii.", "ポイント・プレゼント・購入履歴はサーバーに保存され、このWiiに紐づきます。", "Deine Punkte, Geschenke und Käufe werden auf dem Server gespeichert und dieser Wii zugeordnet.", "Vos points, cadeaux et achats sont enregistrés sur le serveur pour cette Wii.", "Tus puntos, regalos y compras se guardan en el servidor para esta Wii.", "Punti, regali e acquisti sono salvati sul server per questa Wii.", "Je punten, cadeaus en aankopen worden op de server bewaard voor deze Wii.", "您的点数、礼物和购买记录保存在服务器上，并绑定到这台Wii。", "您的點數、禮物和購買記錄儲存在伺服器上，並綁定到這台Wii。", "포인트, 선물, 구매 내역은 서버에 저장되며 이 Wii에 연결됩니다.", "Seus pontos, presentes e compras ficam salvos no servidor para este Wii.")
	row("language", "Language", "言語", "Sprache", "Langue", "Idioma", "Lingua", "Taal", "语言", "語言", "언어", "Idioma")
	row("help_title", "Shroombrew Help", "Shroombrew ヘルプ", "Shroombrew-Hilfe", "Aide Shroombrew", "Ayuda de Shroombrew", "Aiuto Shroombrew", "Shroombrew-hulp", "Shroombrew 帮助", "Shroombrew 說明", "Shroombrew 도움말", "Ajuda do Shroombrew")
	row("help_text", "Pick a collection, choose an app and check its details. Get Wii Points with a card code from the Shroombrew website.", "コレクションからアプリを選び、詳細を確認しましょう。Wiiポイントは、Shroombrewのサイトで入手したカードコードで追加できます。", "Wähle eine Sammlung und eine App und sieh dir die Details an. Wii-Punkte gibt es mit einem Kartencode von der Shroombrew-Website.", "Choisissez une collection puis une appli et consultez ses détails. Obtenez des Points Wii avec un code de carte du site Shroombrew.", "Elige una colección y una app y revisa sus detalles. Consigue Puntos Wii con un código de tarjeta del sitio de Shroombrew.", "Scegli una raccolta e un'app e controlla i dettagli. Ottieni Punti Wii con un codice carta dal sito Shroombrew.", "Kies een collectie en een app en bekijk de details. Krijg Wii-punten met een kaartcode van de Shroombrew-website.", "选择合集和应用，查看详情。可使用Shroombrew网站提供的卡代码获取Wii点数。", "選擇合集和應用程式，查看詳情。可使用Shroombrew網站提供的卡代碼取得Wii點數。", "컬렉션과 앱을 고르고 자세한 정보를 확인하세요. Shroombrew 웹사이트의 카드 코드로 Wii 포인트를 받을 수 있습니다.", "Escolha uma coleção e um app e veja os detalhes. Ganhe Pontos Wii com um código de cartão do site do Shroombrew.")
	row("news_title", "Important Info", "お知らせ", "Wichtige Infos", "Informations importantes", "Información importante", "Informazioni importanti", "Belangrijke info", "重要信息", "重要資訊", "중요 안내", "Informações importantes")
	row("news_text", "Welcome to Shroombrew! Browse homebrew, get Wii Points with card codes and download verified app packages.", "Shroombrewへようこそ！ホームブリューを探し、カードコードでWiiポイントを入手し、確認済みのアプリをダウンロードできます。", "Willkommen bei Shroombrew! Stöbere in Homebrew, hole dir Wii-Punkte mit Kartencodes und lade geprüfte App-Pakete herunter.", "Bienvenue sur Shroombrew ! Parcourez les homebrews, obtenez des Points Wii avec des codes de carte et téléchargez des paquets vérifiés.", "¡Bienvenido a Shroombrew! Explora homebrew, consigue Puntos Wii con códigos de tarjeta y descarga paquetes verificados.", "Benvenuto su Shroombrew! Sfoglia gli homebrew, ottieni Punti Wii con i codici carta e scarica pacchetti verificati.", "Welkom bij Shroombrew! Blader door homebrew, krijg Wii-punten met kaartcodes en download geverifieerde pakketten.", "欢迎使用Shroombrew！浏览自制软件，用卡代码获取Wii点数，并下载经过验证的软件包。", "歡迎使用Shroombrew！瀏覽自製軟體，用卡代碼取得Wii點數，並下載經過驗證的軟體包。", "Shroombrew에 오신 것을 환영합니다! 홈브류를 둘러보고 카드 코드로 Wii 포인트를 받아 검증된 앱을 다운로드하세요.", "Bem-vindo ao Shroombrew! Explore homebrew, ganhe Pontos Wii com códigos de cartão e baixe pacotes verificados.")
	row("activity", "Account Activity", "アカウントの履歴", "Kontoaktivität", "Activité du compte", "Actividad de la cuenta", "Attività dell'account", "Accountactiviteit", "账户记录", "帳戶記錄", "계정 활동", "Atividade da conta")
	row("downloaded", "Titles You've Downloaded", "ダウンロードしたソフト", "Heruntergeladene Titel", "Titres téléchargés", "Títulos descargados", "Titoli scaricati", "Gedownloade titels", "已下载的软件", "已下載的軟體", "다운로드한 소프트웨어", "Títulos baixados")
	row("balance", "Balance: {n} Wii Points", "残高: {n}Wiiポイント", "Guthaben: {n} Wii-Punkte", "Solde : {n} Points Wii", "Saldo: {n} Puntos Wii", "Saldo: {n} Punti Wii", "Saldo: {n} Wii-punten", "余额：{n}Wii点数", "餘額：{n}Wii點數", "잔액: Wii 포인트 {n}점", "Saldo: {n} Pontos Wii")
	row("no_purchases", "No purchases yet.", "購入履歴はまだありません。", "Noch keine Käufe.", "Aucun achat pour l'instant.", "Aún no hay compras.", "Nessun acquisto per ora.", "Nog geen aankopen.", "暂无购买记录。", "暫無購買記錄。", "아직 구매 내역이 없습니다.", "Nenhuma compra ainda.")
	row("dl_again", "{n} points; download again free", "{n}ポイント。再ダウンロードは無料", "{n} Punkte; erneut kostenlos herunterladen", "{n} points ; retéléchargement gratuit", "{n} puntos; descarga de nuevo gratis", "{n} punti; riscarica gratis", "{n} punten; opnieuw gratis downloaden", "{n}点数；可免费重新下载", "{n}點數；可免費重新下載", "{n}포인트; 다시 다운로드는 무료", "{n} pontos; baixe de novo grátis")
}

// tr returns the translation for lang, falling back to English.
func tr(lang, key string) string {
	if v, ok := wiiText[lang][key]; ok && v != "" {
		return v
	}
	return wiiText["en"][key]
}

// trf is tr with {name} placeholders filled from pairs (name, value, ...).
func trf(lang, key string, pairs ...string) string {
	s := tr(lang, key)
	for i := 0; i+1 < len(pairs); i += 2 {
		s = strings.ReplaceAll(s, "{"+pairs[i]+"}", pairs[i+1])
	}
	return s
}

// labelsFor returns every string for a language so a page can embed them.
func labelsFor(lang string) map[string]string {
	out := make(map[string]string, len(wiiText["en"]))
	for k := range wiiText["en"] {
		out[k] = tr(lang, k)
	}
	return out
}
