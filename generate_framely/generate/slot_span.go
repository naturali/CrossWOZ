package generate

import (
	"regexp"
	"strconv"
	"strings"
)

func findSpan(utterance string, slotName string, slotValue string) (fr, to int) {
	if fr := strings.Index(utterance, slotValue); fr != -1 {
		return fr, fr + len(slotValue)
	}

	// 餐厅 <-> 餐馆 可以互换
	if strings.Contains(slotValue, "餐厅") {
		if fr := strings.Index(utterance, strings.Replace(slotValue, "餐厅", "餐馆", -1)); fr != -1 {
			return fr, fr + len(slotValue)
		}
	}
	// bad data
	if slotValue == "精品烤鸭三吃" {
		if fr := strings.Index(utterance, "精品烤鸭三"); fr != -1 {
			return fr, fr + len(slotValue)
		}
	}
	if slotValue == "烧羊肉" {
		if fr := strings.Index(utterance, "烤羊肉"); fr != -1 {
			return fr, fr + len(slotValue)
		}
	}
	if strings.Contains(slotValue, "餐馆") {
		if fr := strings.Index(utterance, strings.Replace(slotValue, "餐馆", "餐厅", -1)); fr != -1 {
			return fr, fr + len(slotValue)
		}
	}

	// 其他复杂的情况
	var slotValueAliasFunc = map[string]func(string) []string{
		"门票":   aliasesFor门票,
		"价格":   aliasesFor价格,
		"游玩时间": aliasesFor游玩时间,
		"评分":   aliasesFor评分,
		"酒店类型": aliasesFor酒店类型,
		"人均消费": aliasFor人均消费,
	}
	if f, ok := slotValueAliasFunc[slotName]; ok {
		aliases := f(slotValue)
		for _, alias := range aliases {
			if fr := strings.Index(utterance, alias); fr != -1 {
				return fr, fr + len(alias)
			}
		}
	}
	return -1, -1

}

func aliasesFor门票(slotValue string) []string {
	// 免费 -> 免票，不花钱
	if slotValue == "免费" {
		return []string{"免票", "不花钱"}
	}
	// 不免票 -> 不免费，花钱，收门票钱
	if slotValue == "不免票" || slotValue == "不免费" {
		return []string{"不免费", "不免票", "花钱", "收门票钱", "收门票费"}
	}
	// XX元以上 -> XX元，XX以上
	if strings.HasSuffix(slotValue, "元以上") {
		return []string{
			strings.TrimSuffix(slotValue, "元以上") + "以上",
			strings.TrimSuffix(slotValue, "以上"),
		}
	}
	// XX-YY元
	if aliases := aliasesForPriceRange(slotValue); len(aliases) > 0 {
		if slotValue == "50-100元" { // 神奇的数据？
			aliases = append(aliases, "20-100元")
		}
		return aliases
	}
	return nil
}

func aliasesFor价格(slotValue string) []string {
	// XX元以上 -> XX以上
	if strings.HasSuffix(slotValue, "元以上") {
		return []string{
			strings.TrimSuffix(slotValue, "元以上") + "以上",
		}
	}
	// XX-YY元
	if aliases := aliasesForPriceRange(slotValue); len(aliases) > 0 {
		return aliases
	}
	return nil
}

func aliasesFor酒店类型(slotValue string) []string {
	// 经济型 -> 经济
	return []string{
		strings.TrimSuffix(slotValue, "型"),
	}
}

func aliasesFor评分(slotValue string) []string {
	// XX分以上 -> XX分，XX以上
	if strings.HasSuffix(slotValue, "分以上") {
		return []string{
			strings.TrimSuffix(slotValue, "以上"),
			strings.TrimSuffix(slotValue, "分以上") + "以上",
			strings.TrimSuffix(slotValue, "分以上") + "是以上", // bad data
		}
	}
	return nil
}

func aliasFor人均消费(slotValue string) []string {
	// XX元以下 -> XX以下
	if strings.HasSuffix(slotValue, "元以下") {
		return []string{
			strings.TrimSuffix(slotValue, "元以下") + "以下",
			strings.TrimSuffix(slotValue, "元以下") + "元一下", // bad data
		}
	}
	// XX-YY元
	if aliases := aliasesForPriceRange(slotValue); len(aliases) > 0 {
		return aliases
	}

	return nil
}

func aliasesFor游玩时间(slotValue string) []string {
	slotValueBytes := []byte(slotValue)
	// XX天-YY天
	regDayRange := regexp.MustCompile(`(\d+)天-(\d+)天`)
	if regDayRange.Match(slotValueBytes) {
		subMatches := regDayRange.FindSubmatch(slotValueBytes)
		if len(subMatches) != 3 {
			return nil
		}
		dayLow := string(subMatches[1])
		dayHigh := string(subMatches[2])
		return []string{
			dayLow + "-" + dayHigh + "天",
		}
	}
	// XX小时-YY小时
	regHourRange := regexp.MustCompile(`(0*\.*\d+)小时-(0*\.*\d+)小时`)
	if regHourRange.Match(slotValueBytes) {
		subMatches := regHourRange.FindSubmatch(slotValueBytes)
		if len(subMatches) != 3 {
			return nil
		}
		hourLow := string(subMatches[1])
		hourHigh := string(subMatches[2])
		return []string{
			hourLow + "-" + hourHigh + "小时",
			hourLow + "到" + hourHigh + "小时",
			hourLow + "小时~" + hourHigh + "小时",
			hourLow + "-" + hourHigh + "个小时",
			hourLow + "、" + hourHigh + "个小时",
		}
	}
	// X小时
	regHour := regexp.MustCompile(`(\d+)小时`)
	if regHour.Match(slotValueBytes) {
		hourStr := string(regHour.FindSubmatch(slotValueBytes)[1])
		aliases := []string{hourStr + "个小时"}
		hour, _ := strconv.Atoi(hourStr)
		if hour > 0 && hour < len(chineseNumbers) {
			aliases = append(aliases, chineseNumbers[hour]+"个小时")
			aliases = append(aliases, chineseNumbers[hour]+"小时")
		}
		return aliases
	}
	return nil
}

var chineseNumbers = []string{"", "一"}

func aliasesForPriceRange(slotValue string) []string {
	// XX-YY元 -> XX到YY元, XX元到YY元, XX到YY元之间，XX元到YY元之间，XX-YY元，XX-YY之间
	reg := regexp.MustCompile(`(\d+)-(\d+)元`)
	subMatches := reg.FindSubmatch([]byte(slotValue))
	if len(subMatches) != 3 {
		return nil
	}
	priceLow := string(subMatches[1])
	priceHigh := string(subMatches[2])
	return []string{
		priceLow + "到" + priceHigh + "元",
		priceLow + "元到" + priceHigh + "元",
		priceLow + "到" + priceHigh + "元之间",
		priceLow + "元到" + priceHigh + "元之间",
		priceLow + "-" + priceHigh,
		priceLow + "-" + priceHigh + "之间",
	}
}
