package database

import (
	"log"

	"Gocorecms/internal/models"
)

// seedYadeaContent organizes the CMS content to mirror yadea.com:
// product categories per series, the current product lineup, newsroom
// categories, static pages and the main/footer navigation menus.
// It is idempotent — every record is looked up by slug/name first, menu
// items are only inserted when the menu is still empty, and drafted copy
// only replaces bodies that still carry the "TODO" placeholder.
func seedYadeaContent() error {
	// ---------- 1. Site identity ----------
	identity := map[string]string{
		"site_name":    "Yadea",
		"site_tagline": "Electrify Your Life",
	}
	for key, value := range identity {
		if err := DB.Model(&models.Setting{}).
			Where("setting_key = ? AND value IN ?", key,
				[]string{"Gocore CMS", "One CMS for every kind of website"}).
			Update("value", value).Error; err != nil {
			return err
		}
	}

	// ---------- 2. Product categories (series) ----------
	type catDef struct {
		Name, Slug, Desc string
		Parent           string // parent slug, empty = top level
	}
	productCats := []catDef{
		{"Electric Motorcycle", "electric-motorcycle", "High-performance electric motorcycles", ""},
		{"Electric Scooter", "electric-scooter", "Urban electric scooters and mopeds", ""},
		{"Electric Bicycle", "electric-bicycle", "Pedal-assist electric bicycles", ""},
		{"K Series", "k-series", "Performance electric motorcycles", "electric-motorcycle"},
		{"V Series", "v-series", "Versatile city scooters", "electric-scooter"},
		{"O Series", "o-series", "Everyday commuter scooters", "electric-scooter"},
		{"G Series", "g-series", "Long-range touring scooters", "electric-scooter"},
	}
	cats := map[string]*models.Category{}
	for _, def := range productCats {
		var parentID *uint
		if def.Parent != "" {
			if p, ok := cats[def.Parent]; ok {
				parentID = &p.ID
			}
		}
		c := models.Category{}
		if err := DB.Where(models.Category{Slug: def.Slug}).
			Attrs(models.Category{Name: def.Name, Description: def.Desc, Type: "product", ParentID: parentID}).
			FirstOrCreate(&c).Error; err != nil {
			return err
		}
		cats[def.Slug] = &c
	}

	// ---------- 3. Newsroom categories ----------
	newsCats := []catDef{
		{"Media Center", "media-center", "Press releases and company news", ""},
		{"Events Center", "events-center", "Exhibitions, launches and event coverage", ""},
		{"Technology", "technology-news", "R&D and technology stories", ""},
	}
	for _, def := range newsCats {
		c := models.Category{}
		if err := DB.Where(models.Category{Slug: def.Slug}).
			Attrs(models.Category{Name: def.Name, Description: def.Desc, Type: "post"}).
			FirstOrCreate(&c).Error; err != nil {
			return err
		}
	}

	// ---------- 4. Product lineup ----------
	type prodDef struct {
		Name, Slug, SKU, Cat, Desc string
	}
	lineup := []prodDef{
		{"YADEA Kemper", "kemper", "YD-KEMPER", "k-series",
			"<p>The Kemper is Yadea's flagship electric motorcycle — built for riders who refuse to compromise between performance and sustainability. A high-output motor delivers instant torque from a standstill, while the race-inspired chassis keeps every corner composed.</p>" +
				"<ul><li>High-performance electric drivetrain with instant torque</li><li>Sport-tuned suspension and braking system</li><li>Fast-charging battery system with intelligent BMS</li><li>Full-color TFT dash with smartphone connectivity</li></ul>" +
				"<p><em>Specifications: top speed [•], range [•], battery [•], charge time [•] — see official spec sheet.</em></p>"},
		{"YADEA Keeness", "keeness", "YD-KEENESS", "k-series",
			"<p>Streetfighter styling meets silent power. The Keeness is a light, agile electric motorcycle designed for the urban rider who wants motorcycle presence with everyday practicality.</p>" +
				"<ul><li>Agile, lightweight frame ideal for city riding</li><li>Responsive electric motor with multiple ride modes</li><li>Keyless start and app-based vehicle status</li><li>LED lighting all around</li></ul>" +
				"<p><em>Specifications: top speed [•], range [•], battery [•], charge time [•] — see official spec sheet.</em></p>"},
		{"YADEA VEKOO", "vekoo", "YD-VEKOO", "v-series",
			"<p>The VEKOO blends contemporary design with smart features, making every city trip effortless. Its balanced chassis and comfortable riding position suit both daily commutes and weekend rides.</p>" +
				"<ul><li>Modern design with premium finish</li><li>Smart connectivity via the Yadea app</li><li>Comfortable two-up seating</li><li>Ample under-seat storage</li></ul>" +
				"<p><em>Specifications: top speed [•], range [•], battery [•], charge time [•] — see official spec sheet.</em></p>"},
		{"YADEA VELAX", "velax", "YD-VELAX", "v-series",
			"<p>Comfort-first commuting. The VELAX pairs a smooth, quiet drivetrain with a plush ride, so the daily commute feels less like a chore and more like a break.</p>" +
				"<ul><li>Smooth, quiet electric drivetrain</li><li>Comfort-tuned suspension</li><li>Practical storage and flat floorboard</li><li>Long-life battery with smart charging</li></ul>" +
				"<p><em>Specifications: top speed [•], range [•], battery [•], charge time [•] — see official spec sheet.</em></p>"},
		{"YADEA OSTA", "osta", "YD-OSTA", "o-series",
			"<p>The OSTA is a dependable everyday scooter that keeps running costs low and reliability high — the sensible choice for riders who just need to get there, every day.</p>" +
				"<ul><li>Reliable, low-maintenance electric drivetrain</li><li>Practical range for daily commuting</li><li>Easy handling for new riders</li><li>Removable battery for convenient charging</li></ul>" +
				"<p><em>Specifications: top speed [•], range [•], battery [•], charge time [•] — see official spec sheet.</em></p>"},
		{"YADEA OVA", "ova", "YD-OVA", "o-series",
			"<p>Compact, nimble and easy to live with, the OVA is made for dense city streets — light enough to manoeuvre anywhere, smart enough to keep you connected.</p>" +
				"<ul><li>Compact frame, easy to park and manoeuvre</li><li>Efficient motor for urban range</li><li>App connectivity and anti-theft alarm</li><li>Bright LED headlight</li></ul>" +
				"<p><em>Specifications: top speed [•], range [•], battery [•], charge time [•] — see official spec sheet.</em></p>"},
		{"YADEA OMEE", "omee", "YD-OMEE", "o-series",
			"<p>The OMEE makes electric riding accessible: a friendly, lightweight scooter with intuitive controls and just-right performance for first-time riders and short-hop commuters.</p>" +
				"<ul><li>Lightweight and beginner-friendly</li><li>Intuitive controls and clear display</li><li>Efficient, economical daily runner</li><li>Removable battery charges anywhere</li></ul>" +
				"<p><em>Specifications: top speed [•], range [•], battery [•], charge time [•] — see official spec sheet.</em></p>"},
		{"YADEA GT80", "gt80", "YD-GT80", "g-series",
			"<p>The flagship of the G Series, the GT80 is built to go the distance. A high-capacity battery and touring-focused ergonomics turn long hauls into easy miles.</p>" +
				"<ul><li>Long-range, high-capacity battery system</li><li>Touring comfort: relaxed ergonomics, generous seat</li><li>Powerful motor for confident overtaking</li><li>Full smart dashboard with navigation support</li></ul>" +
				"<p><em>Specifications: top speed [•], range [•], battery [•], charge time [•] — see official spec sheet.</em></p>"},
		{"YADEA GS80", "gs80", "YD-GS80", "g-series",
			"<p>Sport meets stamina. The GS80 wraps long-range capability in a sharper, sportier package for riders who want their touring with an edge.</p>" +
				"<ul><li>Sport-tuned chassis with long-range battery</li><li>Strong acceleration and stable high-speed cruising</li><li>Dual disc brakes for confident stopping</li><li>Smart app integration</li></ul>" +
				"<p><em>Specifications: top speed [•], range [•], battery [•], charge time [•] — see official spec sheet.</em></p>"},
		{"YADEA GS70", "gs70", "YD-GS70", "g-series",
			"<p>The GS70 strikes the G-Series balance: real touring range in a lighter, more agile scooter that's just as happy threading through town as it is out on the open road.</p>" +
				"<ul><li>Balanced range and agility</li><li>Comfortable for commute and weekend trips alike</li><li>Efficient motor with regenerative braking</li><li>Modern LED lighting and digital dash</li></ul>" +
				"<p><em>Specifications: top speed [•], range [•], battery [•], charge time [•] — see official spec sheet.</em></p>"},
		{"YADEA TROOPER-01 PLUS", "trooper-01-plus", "YD-TROOPER01P", "electric-bicycle",
			"<p>The TROOPER-01 PLUS is Yadea's rugged all-terrain electric bicycle. Fat tires, a torquey mid-drive assist and a long-range battery take you from city streets to gravel trails without breaking a sweat.</p>" +
				"<ul><li>All-terrain fat tires for grip on any surface</li><li>Powerful pedal-assist with multiple levels</li><li>Long-range removable battery</li><li>Front suspension and hydraulic disc brakes</li></ul>" +
				"<p><em>Specifications: motor [•], range [•], battery [•], assist levels [•] — see official spec sheet.</em></p>"},
	}
	for _, def := range lineup {
		var catID *uint
		if c, ok := cats[def.Cat]; ok {
			catID = &c.ID
		}
		p := models.Product{}
		if err := DB.Where(models.Product{Slug: def.Slug}).
			Attrs(models.Product{
				Name:        def.Name,
				SKU:         def.SKU,
				Description: def.Desc,
				Status:      "draft",
				CategoryID:  catID,
			}).FirstOrCreate(&p).Error; err != nil {
			return err
		}
		// Upgrade earlier "TODO" placeholders to the drafted copy, never user edits.
		if err := DB.Model(&models.Product{}).
			Where("slug = ? AND description LIKE 'TODO:%'", def.Slug).
			Update("description", def.Desc).Error; err != nil {
			return err
		}
	}

	// ---------- 5. Static pages ----------
	type pageDef struct{ Title, Slug, Body string }
	pages := []pageDef{
		{"About Us", "about-us",
			"<h1>About Yadea</h1>" +
				"<p>Founded in 2001, Yadea is a global leader in electric two-wheel mobility, developing and manufacturing electric motorcycles, scooters, mopeds and bicycles for riders in more than 100 countries. Our mission is simple: <strong>Electrify Your Life</strong> — making clean, smart, affordable mobility available to everyone.</p>" +
				"<h2>What drives us</h2>" +
				"<p>Every Yadea is the product of relentless investment in research and development — from battery chemistry and motor efficiency to smart connectivity and rider safety. We believe the future of urban transport is electric, and we're building it today.</p>" +
				"<h2>Global presence</h2>" +
				"<p>With manufacturing bases, flagship stores and dealer partners across Asia, Europe, Africa and the Americas, Yadea serves tens of millions of riders worldwide. [Add regional highlights, milestones and figures here.]</p>"},
		{"Technology", "technology",
			"<h1>Technology</h1>" +
				"<p>Yadea's edge is engineered, not claimed. Our in-house R&amp;D teams work across four pillars:</p>" +
				"<h2>Battery systems</h2><p>High-density battery packs with intelligent battery management for longer range, longer life and safer charging. [Detail battery technology and partnerships.]</p>" +
				"<h2>Drive systems</h2><p>Efficient, quiet electric motors tuned for instant torque and low energy consumption. [Detail motor platforms.]</p>" +
				"<h2>Smart connectivity</h2><p>App-connected vehicles with GPS locating, anti-theft alerts, OTA updates and riding analytics.</p>" +
				"<h2>Safety</h2><p>From braking systems to frame engineering and lighting, every model is validated through rigorous durability and safety testing.</p>"},
		{"FAQ", "faq",
			"<h1>Frequently Asked Questions</h1>" +
				"<h2>How far can I ride on one charge?</h2><p>Range depends on the model, rider weight, terrain and temperature. Each product page lists the certified range for that model.</p>" +
				"<h2>How long does charging take?</h2><p>Most models fully charge overnight with the standard charger; models with fast-charge support can reach 80% much quicker. Check your model's spec sheet.</p>" +
				"<h2>Can I remove the battery to charge indoors?</h2><p>Many Yadea models feature removable batteries that can be charged from a regular wall outlet. See your model's manual.</p>" +
				"<h2>Where can I get my Yadea serviced?</h2><p>Any authorized Yadea dealer or service center. Use <a href=\"/find-a-dealer\">Find a Dealer</a> to locate the nearest one.</p>" +
				"<h2>Is there a mobile app?</h2><p>Yes — the Yadea app connects to supported models for vehicle status, locating, anti-theft alerts and software updates.</p>"},
		{"Warranty", "warranty",
			"<h1>Warranty Policy</h1>" +
				"<p>Every Yadea vehicle is covered by a manufacturer's warranty against defects in materials and workmanship. Coverage periods vary by component and region — the terms below are a template; <strong>confirm the official terms for your market before publishing.</strong></p>" +
				"<ul><li>Vehicle frame: [• months]</li><li>Motor: [• months]</li><li>Battery: [• months or charge cycles]</li><li>Controller and charger: [• months]</li><li>Wear items (tires, brake pads, bulbs): not covered</li></ul>" +
				"<h2>What voids the warranty</h2><p>Unauthorized modification, improper charging equipment, water damage beyond the IP rating, and servicing outside authorized centers.</p>" +
				"<h2>How to claim</h2><p>Bring your proof of purchase and registered vehicle to any authorized dealer, or contact <a href=\"/product-support\">Product Support</a>.</p>"},
		{"Product Support", "product-support",
			"<h1>Product Support</h1>" +
				"<p>We're here to keep you riding. Find everything you need to maintain, update and troubleshoot your Yadea.</p>" +
				"<h2>Manuals &amp; downloads</h2><p>[Link user manuals, quick-start guides and spec sheets per model.]</p>" +
				"<h2>Software updates</h2><p>Supported models receive updates over the air via the Yadea app.</p>" +
				"<h2>Service network</h2><p>Authorized service centers handle maintenance, repairs and genuine parts. <a href=\"/find-a-dealer\">Find your nearest dealer</a>.</p>" +
				"<h2>Still need help?</h2><p><a href=\"/contact-us\">Contact us</a> and our support team will get back to you.</p>"},
		{"Product Registration", "product-registration",
			"<h1>Product Registration</h1>" +
				"<p>Register your Yadea within 30 days of purchase to activate your warranty and unlock support services.</p>" +
				"<h2>Why register</h2>" +
				"<ul><li>Activate and track your warranty coverage</li><li>Faster service at authorized centers</li><li>Safety notices and software update alerts</li><li>News and offers for registered owners</li></ul>" +
				"<h2>How to register</h2><p>Have your VIN/frame number and proof of purchase ready, then complete the registration form. [Embed registration form here.]</p>"},
		{"Contact Us", "contact-us",
			"<h1>Contact Us</h1>" +
				"<p>Questions about a product, an order or a partnership? We'd love to hear from you.</p>" +
				"<ul><li><strong>Customer support:</strong> [support email / phone]</li><li><strong>Press &amp; media:</strong> [press email]</li><li><strong>Business &amp; partnerships:</strong> [business email]</li><li><strong>Head office:</strong> [address]</li></ul>" +
				"<p>Or use the contact form below and we'll get back to you within [•] business days. [Embed contact form — posts to /api/v1/public/contact.]</p>"},
		{"Find a Dealer", "find-a-dealer",
			"<h1>Find a Dealer</h1>" +
				"<p>Yadea vehicles are sold and serviced through a network of authorized dealers. Visit a showroom to see the lineup, take a test ride and get expert advice.</p>" +
				"<p>[Embed dealer locator map / searchable dealer list here.]</p>" +
				"<p>Can't find a dealer near you? <a href=\"/contact-us\">Contact us</a> — or if you run a store, consider <a href=\"/dealer\">becoming a dealer</a>.</p>"},
		{"Test Ride", "test-drive",
			"<h1>Book a Test Ride</h1>" +
				"<p>The best way to understand a Yadea is to ride one. Book a free test ride at a dealer near you.</p>" +
				"<h2>How it works</h2>" +
				"<ol><li>Choose the model you want to try</li><li>Pick a participating dealer and time slot</li><li>Bring a valid license (where required) — helmet provided</li></ol>" +
				"<p>[Embed test ride booking form here.]</p>"},
		{"Become a Dealer", "dealer",
			"<h1>Become a Yadea Dealer</h1>" +
				"<p>Join one of the world's largest electric two-wheeler networks. As a Yadea partner you get a proven product lineup, strong margins, marketing support and full technical training.</p>" +
				"<h2>What we look for</h2>" +
				"<ul><li>Retail or automotive experience in your market</li><li>Showroom and basic service capability</li><li>Commitment to the electric mobility transition</li></ul>" +
				"<h2>Apply</h2><p>Tell us about your business and market. [Embed dealer application form here.] Our regional team will follow up within [•] business days.</p>"},
		{"Investor Relations", "investor",
			"<h1>Investor Relations</h1>" +
				"<p>Yadea Group Holdings Ltd. is listed on the Hong Kong Stock Exchange. This section provides shareholders and analysts with financial reports, announcements and corporate governance information.</p>" +
				"<h2>Resources</h2>" +
				"<ul><li>Annual and interim reports [link]</li><li>Results announcements [link]</li><li>Corporate governance [link]</li><li>IR contact: [email]</li></ul>" +
				"<p><em>Verify listing details and link official filings before publishing.</em></p>"},
		{"Career", "career",
			"<h1>Careers at Yadea</h1>" +
				"<p>Help us electrify the way the world moves. We hire engineers, designers, marketers and operations talent across our global offices and manufacturing bases.</p>" +
				"<h2>Why Yadea</h2>" +
				"<ul><li>Work on products used by millions of riders</li><li>Global teams, real ownership, fast growth</li><li>Deep investment in R&amp;D and sustainability</li></ul>" +
				"<h2>Open positions</h2><p>[List openings or link to the careers portal here.]</p>"},
		{"Privacy Policy", "privacy-policy",
			"<h1>Privacy Policy</h1>" +
				"<p><em>Template — must be reviewed by legal counsel for your jurisdictions before publishing.</em></p>" +
				"<h2>Data we collect</h2><p>Account details, order information, vehicle telemetry (for connected models), and website analytics.</p>" +
				"<h2>How we use it</h2><p>To provide products and services, process orders, deliver app features, improve products and, with consent, send marketing communications.</p>" +
				"<h2>Sharing</h2><p>With service providers, dealers and logistics partners as needed to serve you; never sold to third parties.</p>" +
				"<h2>Your rights</h2><p>Access, correction, deletion and portability requests: [privacy email].</p>"},
		{"Terms of Use", "terms-of-use",
			"<h1>Terms of Use</h1>" +
				"<p><em>Template — must be reviewed by legal counsel before publishing.</em></p>" +
				"<h2>Use of this website</h2><p>Content is provided for information only and may change without notice. Product availability, colors and specifications vary by market.</p>" +
				"<h2>Intellectual property</h2><p>All trademarks, images and content are the property of Yadea or its licensors and may not be reproduced without permission.</p>" +
				"<h2>Limitation of liability</h2><p>[Standard limitation clause.]</p>" +
				"<h2>Governing law</h2><p>[Jurisdiction.]</p>"},
	}
	var author models.User
	DB.Order("id").First(&author)
	for _, def := range pages {
		pg := models.Page{}
		if err := DB.Where(models.Page{Slug: def.Slug}).
			Attrs(models.Page{
				Title:    def.Title,
				Body:     def.Body,
				Status:   "draft",
				AuthorID: author.ID,
			}).FirstOrCreate(&pg).Error; err != nil {
			return err
		}
		// Upgrade earlier "TODO" placeholders to the drafted copy, never user edits.
		if err := DB.Model(&models.Page{}).
			Where("slug = ? AND body LIKE '%TODO: add content.%'", def.Slug).
			Update("body", def.Body).Error; err != nil {
			return err
		}
	}

	// ---------- 6. Navigation menus ----------
	menus := map[string][]itemDef{
		"main": {
			{Label: "Product", URL: "#", Children: []itemDef{
				{Label: "K Series", URL: "/electric-motorcycle"},
				{Label: "V Series", URL: "/electric-scooter?series=v"},
				{Label: "O Series", URL: "/electric-scooter?series=o"},
				{Label: "G Series", URL: "/electric-scooter?series=g"},
				{Label: "Electric Bicycle", URL: "/electric-bicycle"},
			}},
			{Label: "Technology", URL: "/technology"},
			{Label: "Newsroom", URL: "#", Children: []itemDef{
				{Label: "Media Center", URL: "/news-and-events"},
				{Label: "Events Center", URL: "/news-and-events?option=events-center"},
			}},
			{Label: "Support", URL: "#", Children: []itemDef{
				{Label: "FAQ", URL: "/faq"},
				{Label: "Warranty", URL: "/warranty"},
				{Label: "Product Support", URL: "/product-support"},
				{Label: "Product Registration", URL: "/product-registration"},
			}},
			{Label: "Company", URL: "#", Children: []itemDef{
				{Label: "About Us", URL: "/about-us"},
				{Label: "Investor Relations", URL: "/investor"},
				{Label: "Career", URL: "/career"},
			}},
			{Label: "Contact", URL: "#", Children: []itemDef{
				{Label: "Contact Us", URL: "/contact-us"},
				{Label: "Find a Dealer", URL: "/find-a-dealer"},
				{Label: "Test Ride", URL: "/test-drive"},
				{Label: "Become a Dealer", URL: "/dealer"},
			}},
		},
		"footer": {
			{Label: "About Us", URL: "/about-us"},
			{Label: "Newsroom", URL: "/news-and-events"},
			{Label: "FAQ", URL: "/faq"},
			{Label: "Warranty", URL: "/warranty"},
			{Label: "Contact Us", URL: "/contact-us"},
			{Label: "Privacy Policy", URL: "/privacy-policy"},
			{Label: "Terms of Use", URL: "/terms-of-use"},
			{Label: "Facebook", URL: "https://www.facebook.com/yadea.official", Target: "_blank"},
			{Label: "Instagram", URL: "https://www.instagram.com/yadea.global/", Target: "_blank"},
			{Label: "X (Twitter)", URL: "https://twitter.com/YadeaGlobal/", Target: "_blank"},
			{Label: "TikTok", URL: "https://www.tiktok.com/@yadea_official", Target: "_blank"},
			{Label: "LinkedIn", URL: "https://www.linkedin.com/company/yadea", Target: "_blank"},
		},
	}
	for name, items := range menus {
		var menu models.Menu
		if err := DB.Where(models.Menu{Name: name}).FirstOrCreate(&menu).Error; err != nil {
			return err
		}
		var itemCount int64
		DB.Model(&models.MenuItem{}).Where("menu_id = ?", menu.ID).Count(&itemCount)
		if itemCount > 0 {
			continue // menu already curated — don't touch it
		}
		for i, item := range items {
			if err := createMenuItem(menu.ID, nil, item, i); err != nil {
				return err
			}
		}
		log.Printf("seeded %q menu with %d top-level items", name, len(items))
	}

	return nil
}

// itemDef describes one navigation link; Children nest one level below it.
type itemDef struct {
	Label, URL, Target string
	Children           []itemDef
}

func createMenuItem(menuID uint, parentID *uint, def itemDef, sort int) error {
	target := def.Target
	if target == "" {
		target = "_self"
	}
	item := models.MenuItem{
		MenuID:    menuID,
		ParentID:  parentID,
		Label:     def.Label,
		URL:       def.URL,
		Target:    target,
		SortOrder: sort,
	}
	if err := DB.Create(&item).Error; err != nil {
		return err
	}
	for i, child := range def.Children {
		if err := createMenuItem(menuID, &item.ID, child, i); err != nil {
			return err
		}
	}
	return nil
}
