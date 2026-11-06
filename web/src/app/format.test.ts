import { describe, expect, it } from 'vitest'
import {
  addDays,
  calendarDate,
  courseName,
  coverURL,
  fromUnit,
  inUnit,
  shelfSize,
  day,
  daysLate,
  endOf,
  lastDay,
  lengthIn,
  money,
  names,
  numberRanges,
  sourceLabel,
  termLabel,
  today,
} from './format'

describe('format', () => {
  it('picks the smallest cover rendition wide enough, else the master', () => {
    expect(coverURL({ id: 'abc', widths: [200, 400] }, 150)).toBe('/media/abc/w200.jpg')
    expect(coverURL({ id: 'abc', widths: [200, 400] }, 300)).toBe('/media/abc/w400.jpg')
    expect(coverURL({ id: 'abc', widths: [200] }, 600)).toBe('/media/abc/master.jpg')
    expect(coverURL({ id: 'abc', widths: null }, 100)).toBe('/media/abc/master.jpg')
  })

  it('writes a measurement in the chosen unit, and nothing when it is not known', () => {
    expect(lengthIn(240, 'cm')).toBe('24 cm')
    expect(lengthIn(201, 'in')).toBe('7.9 in')
    expect(lengthIn(0, 'in')).toBe('')
  })

  it('writes a price only when it is known', () => {
    expect(money(-1)).toBe('')
    expect(money(799)).toContain('7.99')
  })

  it('writes a date as precisely as it is known', () => {
    expect(calendarDate('2014')).toBe('2014')
    expect(calendarDate('')).toBe('')
    expect(calendarDate('2014-06')).toContain('2014')
  })

  it('ends a day at the next midnight on the device and reads it back', () => {
    const end = new Date(endOf('2026-12-31'))
    expect([end.getFullYear(), end.getMonth(), end.getDate(), end.getHours()]).toEqual([
      2027, 0, 1, 0,
    ])
    expect(lastDay(endOf('2026-12-31'))).toBe(calendarDate('2026-12-31'))
    expect(day('')).toBe('Never')
  })

  it('counts days on the device clock', () => {
    expect(today(new Date(2026, 10, 4, 23, 30))).toBe('2026-11-04')
    expect(addDays('2026-12-25', 14)).toBe('2027-01-08')
    expect(daysLate('2026-11-01', '2026-11-04')).toBe(3)
    expect(daysLate('2026-11-04', '2026-11-04')).toBe(0)
    expect(daysLate('', '2026-11-04')).toBe(0)
    // Across the spring clock change, a day is still a day.
    expect(daysLate('2027-03-13', '2027-03-15')).toBe(2)
  })

  it('writes a course as a transcript does', () => {
    const c = { department: 'MATH', number: '241', title: 'Calculus III', term: 'fall', year: 2004 }
    expect(courseName(c)).toBe('MATH 241 Calculus III')
    expect(termLabel(c)).toBe('Fall 2004')
    expect(termLabel({ ...c, term: 'unknown' })).toBe('2004')
    expect(termLabel({ ...c, term: 'unknown', year: 0 })).toBe('')
  })

  it('joins names for reading', () => {
    const people = [
      { name: 'A', role: 'author' },
      { name: 'B', role: 'author' },
      { name: 'C', role: 'author' },
      { name: 'D', role: 'translator' },
    ]
    expect(names(people, 'author')).toBe('A, B, and C')
    expect(names(people, 'translator')).toBe('D')
    expect(names(null, 'author')).toBe('')
  })

  it('names sources in words', () => {
    expect(sourceLabel('loc')).toBe('Library of Congress')
    expect(sourceLabel('hardcover')).toBe('Hardcover')
    expect(sourceLabel(undefined)).toBe('')
  })
})

describe('volume numbers', () => {
  it('joins runs of whole numbers and keeps the rest apart', () => {
    expect(numberRanges(['1', '2', '3', '5', '7', '8', '2.5'])).toBe('1–3, 5, 7–8, 2.5')
    expect(numberRanges(['4'])).toBe('4')
    expect(numberRanges([])).toBe('')
  })
})

describe('shelf units', () => {
  it('turns millimetres into inches or centimetres and back', () => {
    expect(inUnit(800, 'in')).toBe(31.5)
    expect(inUnit(800, 'cm')).toBe(80)
    expect(fromUnit(31.5, 'in')).toBe(800)
    expect(fromUnit(11, 'in')).toBe(279)
    expect(shelfSize({ widthMm: 800, heightMm: 279, depthMm: 254 }, 'in')).toBe('31.5 × 11 × 10 in')
    expect(
      shelfSize(
        { widthMm: 800, heightMm: 279, depthMm: 254, braceHeightMm: 38, braceWidthMm: 19 },
        'in',
      ),
    ).toBe('31.5 × 11 × 10 in, braces 1.5 × 0.75 in')
  })

  it('gives a field the plainest number that keeps the same millimetres', () => {
    for (const typed of [11, 31.5, 1.5, 1.25, 0.75, 10.9, 7.95]) {
      expect(inUnit(fromUnit(typed, 'in'), 'in', 2)).toBe(typed)
    }
    expect(inUnit(279, 'cm', 2)).toBe(27.9)
    expect(inUnit(280, 'cm', 2)).toBe(28)
    for (let mm = 1; mm <= 1000; mm++) {
      expect(fromUnit(inUnit(mm, 'in', 2), 'in')).toBe(mm)
      expect(fromUnit(inUnit(mm, 'cm', 2), 'cm')).toBe(mm)
    }
  })

  it('reads to a tenth, keeping quarter inches that are exact', () => {
    expect(inUnit(212, 'in')).toBe(8.3)
    expect(inUnit(35, 'in')).toBe(1.4)
    expect(inUnit(19, 'in')).toBe(0.75)
    expect(inUnit(32, 'in')).toBe(1.25)
    expect(inUnit(279, 'in')).toBe(11)
  })
})
