import React, { useState, useEffect } from 'react';
import { Country, City } from 'country-state-city';

interface CountryCitySelectorProps {
  countryValue: string;
  cityValue: string;
  onCountryChange: (value: string) => void;
  onCityChange: (value: string) => void;
}

export const CountryCitySelector: React.FC<CountryCitySelectorProps> = ({
  countryValue,
  cityValue,
  onCountryChange,
  onCityChange,
}) => {
  const [countries] = useState(() => Country.getAllCountries());
  const [cities, setCities] = useState<any[]>([]);

  // Find the ISO code for the current country name (trim to avoid mismatches)
  const currentCountryISO = React.useMemo(() => {
    if (!countryValue) return '';
    const cleanName = countryValue.trim();
    const found = countries.find(c => c.name === cleanName);
    return found ? found.isoCode : '';
  }, [countryValue, countries]);

  // Load cities whenever the country ISO changes
  useEffect(() => {
    if (currentCountryISO) {
      const countryCities = City.getCitiesOfCountry(currentCountryISO) || [];
      // Use unique names for city list
      const uniqueNames = Array.from(new Set(countryCities.map(c => c.name)));
      const filteredCities = uniqueNames.map(name => countryCities.find(c => c.name === name));
      setCities(filteredCities);
    } else {
      setCities([]);
    }
  }, [currentCountryISO]);

  return (
    <div className="form-row">
      <div className="form-group">
        <label>País</label>
        <select
          value={currentCountryISO} // Controlled by ISO code
          onChange={(e) => {
            const iso = e.target.value;
            const country = countries.find(c => c.isoCode === iso);
            onCountryChange(country ? country.name : '');
          }}
          className="form-control-select"
        >
          <option value="">Seleccionar país...</option>
          {countries.map((c) => (
            <option key={c.isoCode} value={c.isoCode}>
              {c.name}
            </option>
          ))}
        </select>
      </div>
      <div className="form-group">
        <label>Ciudad</label>
        <select
          value={cityValue || ""} // Keep city as name as there's no ISO for cities
          onChange={(e) => onCityChange(e.target.value)}
          disabled={!currentCountryISO}
          className="form-control-select"
        >
          <option value="">Seleccionar ciudad...</option>
          {cities.map((city, index) => (
            <option key={`${city?.name}-${index}`} value={city?.name}>
              {city?.name}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
};
