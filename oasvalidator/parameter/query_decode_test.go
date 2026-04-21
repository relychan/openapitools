// Copyright 2026 RelyChan Pte. Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parameter

import (
	"net/url"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/openapitools/oaschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeQueryValuesFromParameters(t *testing.T) {
	testCases := []struct {
		name       string
		value      string
		parameters []*highv3.Parameter
		expected   map[string]any
	}{
		{
			name:  "form_explode_single",
			value: "id=3",
			expected: map[string]any{
				"id": int64(3),
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(true),
					Style:   oaschema.EncodingStyleForm.String(),
					Schema: base.CreateSchemaProxy(&base.Schema{
						Type: []string{oaschema.Integer},
					}),
				},
			},
		},
		{
			name:  "form_single",
			value: "id=3",
			expected: map[string]any{
				"id": []string{"3"},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(false),
					Style:   oaschema.EncodingStyleForm.String(),
				},
			},
		},
		{
			name:  "form_multiple",
			value: "id=3,4,5",
			expected: map[string]any{
				"id": []string{"3", "4", "5"},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(false),
					Style:   oaschema.EncodingStyleForm.String(),
				},
			},
		},
		{
			name: "form_explode_multiple",
			expected: map[string]any{
				"id": []string{"3", "4", "5"},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(true),
					Style:   oaschema.EncodingStyleForm.String(),
				},
			},
			value: "id=3&id=4&id=5",
		},
		{
			name:  "form_object",
			value: "id=role,admin",
			expected: map[string]any{
				"id": map[string]any{
					"role": "admin",
				},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(false),
					Style:   oaschema.EncodingStyleForm.String(),
					Schema: base.CreateSchemaProxy(&base.Schema{
						Type: []string{oaschema.Object},
					}),
				},
			},
		},
		{
			name:  "form_explode_object",
			value: "role=admin",
			expected: map[string]any{
				"id": map[string]any{
					"role": "admin",
				},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(true),
					Style:   oaschema.EncodingStyleForm.String(),
					Schema: base.CreateSchemaProxy(&base.Schema{
						Type: []string{oaschema.Object},
						Properties: func() *orderedmap.Map[string, *base.SchemaProxy] {
							result := orderedmap.New[string, *base.SchemaProxy]()
							result.Set("role", base.CreateSchemaProxy(&base.Schema{
								Type: []string{oaschema.String},
							}))

							return result
						}(),
					}),
				},
			},
		},
		{
			name:  "spaceDelimited_array",
			value: "id=3+4+5",
			expected: map[string]any{
				"id": []any{int64(3), int64(4), int64(5)},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(false),
					Style:   oaschema.EncodingStyleSpaceDelimited.String(),
					Schema: base.CreateSchemaProxy(&base.Schema{
						Type: []string{oaschema.Array},
						Items: &base.DynamicValue[*base.SchemaProxy, bool]{
							A: base.CreateSchemaProxy(&base.Schema{
								Type: []string{oaschema.Integer},
							}),
						},
					}),
				},
			},
		},
		{
			name:  "spaceDelimited_object",
			value: "color=G+200+R+100",
			expected: map[string]any{
				"color": map[string]any{
					"R": float64(100),
					"G": float64(200),
				},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "color",
					In:      oaschema.InQuery.String(),
					Explode: new(false),
					Style:   oaschema.EncodingStyleSpaceDelimited.String(),
					Schema: base.CreateSchemaProxy(&base.Schema{
						Type: []string{oaschema.Object},
						Properties: func() *orderedmap.Map[string, *base.SchemaProxy] {
							result := orderedmap.New[string, *base.SchemaProxy]()
							result.Set("R", base.CreateSchemaProxy(&base.Schema{
								Type: []string{oaschema.Number},
							}))
							result.Set("G", base.CreateSchemaProxy(&base.Schema{
								Type: []string{oaschema.Number},
							}))

							return result
						}(),
						Items: &base.DynamicValue[*base.SchemaProxy, bool]{
							A: base.CreateSchemaProxy(&base.Schema{
								Type: []string{oaschema.Integer},
							}),
						},
					}),
				},
			},
		},
		{
			name:  "spaceDelimited_explode_array",
			value: "id=3&id=4&id=5",
			expected: map[string]any{
				"id": []any{"3", "4", "5"},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(true),
					Style:   oaschema.EncodingStyleSpaceDelimited.String(),
					Schema: base.CreateSchemaProxy(&base.Schema{
						Type: []string{oaschema.Array},
					}),
				},
			},
		},
		{
			name:  "pipeDelimited_array",
			value: "id=3%7C4%7C5",
			expected: map[string]any{
				"id": []string{"3", "4", "5"},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(false),
					Style:   oaschema.EncodingStylePipeDelimited.String(),
				},
			},
		},
		{
			name:  "pipeDelimited_explode_array",
			value: "id=3&id=4&id=5",
			expected: map[string]any{
				"id": []string{"3", "4", "5"},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.EncodingStylePipeDelimited.String(),
					Explode: new(true),
					Style:   oaschema.EncodingStyleSpaceDelimited.String(),
				},
			},
		},
		{
			name:  "pipeDelimited_object",
			value: "color=G%7C200%7CR%7C100",
			expected: map[string]any{
				"color": map[string]any{
					"R": "100",
					"G": "200",
				},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "color",
					In:      oaschema.InQuery.String(),
					Explode: new(false),
					Style:   oaschema.EncodingStylePipeDelimited.String(),
					Schema: base.CreateSchemaProxy(&base.Schema{
						Type: []string{oaschema.Object},
					}),
				},
			},
		},
		{
			name:  "deepObject_array_explode",
			value: "id[]=3&id[]=4&id[]=5",
			expected: map[string]any{
				"id": []string{"3", "4", "5"},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(true),
					Style:   oaschema.EncodingStyleDeepObject.String(),
				},
			},
		},
		{
			name:  "deepObject_object_explode",
			value: "color%5BG%5D=200&color%5BR%5D=100",
			expected: map[string]any{
				"color": map[string]any{
					"R": "100",
					"G": "200",
				},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "color",
					In:      oaschema.InQuery.String(),
					Explode: new(true),
					Style:   oaschema.EncodingStyleDeepObject.String(),
				},
			},
		},
		{
			name: "deepObject_array_object",
			expected: map[string]any{
				"role": []any{
					map[string]any{
						"user": []any{
							[]string{"admin", "anonymous"},
						},
					},
				},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "role",
					In:      oaschema.InQuery.String(),
					Explode: new(true),
					Style:   oaschema.EncodingStyleDeepObject.String(),
				},
			},
			value: "role[0][user][0]=admin&role[0][user][0]=anonymous",
		},
		{
			name:  "deepObject_explode_array_object",
			value: "id[role][0][user][0][]=admin&id[role][0][user][0][]=anonymous",
			expected: map[string]any{
				"id": map[string]any{
					"role": []any{
						map[string]any{
							"user": []any{
								[]string{"admin", "anonymous"},
							},
						},
					},
				},
			},
			parameters: []*highv3.Parameter{
				{
					Name:    "id",
					In:      oaschema.InQuery.String(),
					Explode: new(true),
					Style:   oaschema.EncodingStyleDeepObject.String(),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			qValues, err := url.ParseQuery(tc.value)
			require.NoError(t, err)

			result, errs := DecodeQueryFromParameters(tc.parameters, qValues)
			assert.Equal(t, 0, len(errs), errs)
			require.Equal(t, tc.expected, result, qValues)
		})
	}
}

// BenchmarkDecodeQueryFromParameters/parse_deep_object-11         	 1323890	       914.3 ns/op	    1036 B/op	      34 allocs/op
// BenchmarkDecodeQueryFromParameters/decode_deep_object-11        	  802162	      1435 ns/op	    2212 B/op	      48 allocs/op
// BenchmarkDecodeQueryFromParameters/parse_deep_object_complex-11 	   14733	     81442 ns/op	   63993 B/op	    1966 allocs/op
func BenchmarkDecodeQueryFromParameters(b *testing.B) {
	value := "id[role][0][user][0][]=admin&id[role][0][user][0][]=anonymous&name=foo&bar=baz"
	parameters := []*highv3.Parameter{
		{
			Name:    "id",
			In:      oaschema.InQuery.String(),
			Explode: new(true),
			Style:   oaschema.EncodingStyleDeepObject.String(),
		},
		{
			Name:    "name",
			In:      oaschema.InQuery.String(),
			Explode: new(true),
			Style:   oaschema.EncodingStyleForm.String(),
		},
		{
			Name:    "bar",
			In:      oaschema.InQuery.String(),
			Explode: new(true),
			Style:   oaschema.EncodingStyleSpaceDelimited.String(),
		},
	}

	qValues, err := url.ParseQuery(value)
	require.NoError(b, err)

	b.Run("parse_deep_object", func(b *testing.B) {
		for b.Loop() {
			_, errs := parseDeepObjectNodes(qValues)
			if len(errs) > 0 {
				panic(errs)
			}
		}
	})

	b.Run("decode_deep_object", func(b *testing.B) {
		for b.Loop() {
			_, errs := DecodeQueryFromParameters(parameters, qValues)
			if len(errs) > 0 {
				panic(errs)
			}
		}
	})

	b.Run("parse_deep_object_complex", func(b *testing.B) {
		complexValue := "payment_method_options[bacs_debit][setup_future_usage]=off_session&payment_intent_data[shipping][address][line1]=3YghEmysVn&consent_collection[payment_method_reuse_agreement][position]=auto&payment_method_options[au_becs_debit][setup_future_usage]=none&payment_method_options[customer_balance][bank_transfer][eu_bank_transfer][country]=mzrVWAjBTc&payment_method_options[acss_debit][verification_method]=instant&payment_intent_data[transfer_group]=XKfPQPVhOT&line_items[0][price_data][currency]=euIDO8C4A7&invoice_creation[invoice_data][footer]=OAELqbYbKV&payment_method_options[us_bank_account][verification_method]=automatic&payment_method_options[cashapp][setup_future_usage]=off_session&payment_method_options[konbini][expires_after_days]=664583520&subscription_data[default_tax_rates][]=b3jgFBJq4f&line_items[0][price_data][product_data][tax_code]=PzbIHvqWJp&line_items[0][adjustable_quantity][minimum]=905088217&currency=oVljMB8lon&invoice_creation[invoice_data][issuer][account]=aqOwDzxnyg&success_url=hDTwi34TAz&payment_method_options[giropay][setup_future_usage]=none&payment_method_options[oxxo][setup_future_usage]=none&payment_method_options[acss_debit][currency]=usd&payment_method_options[acss_debit][mandate_options][payment_schedule]=sporadic&payment_intent_data[shipping][name]=mJYqgRIh3S&subscription_data[transfer_data][amount_percent]=1.5805719275050356&custom_fields[0][label][custom]=uabTz3xzdn&customer_update[name]=never&payment_method_options[acss_debit][mandate_options][interval_description]=iMgay8S9If&automatic_tax[liability][type]=self&payment_intent_data[capture_method]=manual&custom_fields[0][dropdown][options][0][label]=W3oysCi31d&custom_fields[0][text][maximum_length]=331815114&custom_text[terms_of_service_acceptance][message]=zGLTTZItPl&invoice_creation[enabled]=true&shipping_options[0][shipping_rate]=5PAjqTpMjw&shipping_options[0][shipping_rate_data][delivery_estimate][minimum][unit]=day&payment_method_types[]=acss_debit&payment_intent_data[application_fee_amount]=2033958571&custom_text[after_submit][message]=b7ifuedi9S&shipping_options[0][shipping_rate_data][delivery_estimate][minimum][value]=1640284987&payment_method_options[afterpay_clearpay][setup_future_usage]=none&payment_method_options[alipay][setup_future_usage]=none&payment_method_options[us_bank_account][financial_connections][permissions][]=ownership&mode=payment&line_items[0][quantity]=968305911&return_url=YgIdKykEHC&shipping_options[0][shipping_rate_data][delivery_estimate][maximum][value]=479399576&payment_method_options[paypal][reference]=ulLn2NXA1P&subscription_data[transfer_data][destination]=wzJ3U1Tyhd&customer=mT4BKOSu9s&submit_type=donate&payment_method_options[boleto][setup_future_usage]=none&payment_method_options[acss_debit][mandate_options][transaction_type]=business&payment_intent_data[shipping][address][city]=v6nZI33cUt&custom_fields[0][type]=dropdown&invoice_creation[invoice_data][custom_fields][0][name]=LBlZjJ4gEy&shipping_options[0][shipping_rate_data][tax_behavior]=exclusive&customer_email=1xiCJ8M7Pr&invoice_creation[invoice_data][issuer][type]=account&payment_method_options[customer_balance][funding_type]=bank_transfer&payment_intent_data[shipping][address][state]=ILODDWP1IP&subscription_data[proration_behavior]=create_prorations&line_items[0][price_data][product_data][name]=ak6UVjXl1B&invoice_creation[invoice_data][custom_fields][0][value]=EWoKgkV3fg&shipping_options[0][shipping_rate_data][type]=fixed_amount&payment_method_options[link][setup_future_usage]=none&expand[]=ZBxEXz7SN0&subscription_data[invoice_settings][issuer][type]=account&payment_method_collection=always&customer_update[address]=never&payment_method_options[wechat_pay][setup_future_usage]=none&customer_creation=always&payment_method_options[card][statement_descriptor_suffix_kanji]=Y57zexRcIH&payment_method_options[p24][setup_future_usage]=none&locale=auto&line_items[0][price_data][product_data][images][]=gE5K8MOzRc&payment_method_options[us_bank_account][setup_future_usage]=none&payment_intent_data[on_behalf_of]=mpkGzXu3st&custom_fields[0][label][type]=custom&custom_fields[0][optional]=false&line_items[0][price_data][tax_behavior]=inclusive&billing_address_collection=auto&invoice_creation[invoice_data][rendering_options][amount_tax_display]=exclude_tax&shipping_options[0][shipping_rate_data][fixed_amount][currency]=KkRL3jvZMO&payment_method_options[grabpay][setup_future_usage]=none&ui_mode=hosted&payment_intent_data[transfer_data][destination]=LrcNMrJPkO&shipping_options[0][shipping_rate_data][tax_code]=NKSQxYdCfO&payment_method_options[affirm][setup_future_usage]=none&payment_method_options[paypal][setup_future_usage]=none&payment_method_options[acss_debit][mandate_options][custom_mandate_url]=FZwPtJKktL&automatic_tax[liability][account]=gW7D0WhP9C&custom_fields[0][numeric][maximum_length]=678468035&custom_fields[0][text][minimum_length]=1689246767&line_items[0][price_data][recurring][interval_count]=592739346&client_reference_id=ZcJeCf6JAa&line_items[0][price_data][unit_amount]=945322526&line_items[0][adjustable_quantity][maximum]=1665059759&discounts[0][coupon]=tOlEXiZKv9&shipping_address_collection[allowed_countries][]=AC&payment_method_options[paypal][risk_correlation_id]=fj1J6Nux6P&payment_method_options[acss_debit][setup_future_usage]=off_session&payment_method_options[konbini][setup_future_usage]=none&payment_intent_data[statement_descriptor_suffix]=dtPJwyuc4i&payment_intent_data[setup_future_usage]=off_session&subscription_data[on_behalf_of]=oGsMnSifXV&allow_promotion_codes=true&custom_fields[0][key]=5ZeyjIHLn8&custom_text[submit][message]=vGcSz5eSlo&setup_intent_data[on_behalf_of]=165u5Fvodj&discounts[0][promotion_code]=Xknj8juRnm&customer_update[shipping]=auto&shipping_options[0][shipping_rate_data][delivery_estimate][maximum][unit]=week&payment_method_options[oxxo][expires_after_days]=1925345768&payment_intent_data[receipt_email]=LxJLYGjJ4r&subscription_data[trial_settings][end_behavior][missing_payment_method]=create_invoice&after_expiration[recovery][enabled]=true&payment_method_configuration=uwYSwIZP4V&invoice_creation[invoice_data][account_tax_ids][]=dev8vFF6xG&shipping_options[0][shipping_rate_data][fixed_amount][amount]=2040036333&payment_method_options[paypal][capture_method]=manual&payment_method_options[paypal][preferred_locale]=cs-CZ&payment_intent_data[shipping][address][country]=O8MBVcia7c&after_expiration[recovery][allow_promotion_codes]=true&custom_text[shipping_address][message]=XeD5TkmC8k&line_items[0][price_data][recurring][interval]=day&line_items[0][price_data][product]=xilQ2QDVdA&line_items[0][dynamic_tax_rates][]=jMMvH8TmQD&payment_method_options[card][setup_future_usage]=on_session&payment_method_options[customer_balance][bank_transfer][type]=gb_bank_transfer&payment_method_options[sepa_debit][setup_future_usage]=none&automatic_tax[enabled]=false&consent_collection[terms_of_service]=required&payment_method_options[fpx][setup_future_usage]=none&payment_method_options[us_bank_account][financial_connections][prefetch][]=transactions&payment_intent_data[transfer_data][amount]=94957585&payment_method_options[bancontact][setup_future_usage]=none&payment_intent_data[statement_descriptor]=JCOo6lU8Fy&line_items[0][tax_rates][]=Ts1bPAoT0T&line_items[0][price]=fR6vnvprv8&setup_intent_data[description]=U9qFTQnt1W&redirect_on_completion=never&shipping_options[0][shipping_rate_data][display_name]=PXozGQQnBA&payment_method_options[card][installments][enabled]=true&payment_method_options[p24][tos_shown_and_accepted]=true&payment_method_options[wechat_pay][app_id]=9Pu0d1pZ2r&payment_method_options[wechat_pay][client]=ios&payment_method_options[boleto][expires_after_days]=953467886&payment_method_options[eps][setup_future_usage]=none&payment_method_options[acss_debit][mandate_options][default_for][]=invoice&subscription_data[trial_end]=606476058&custom_fields[0][numeric][minimum_length]=2134997439&line_items[0][price_data][product_data][description]=DQECtJEsLI&consent_collection[promotions]=auto&payment_method_options[swish][reference]=rXJq1EX4rc&payment_intent_data[shipping][carrier]=P8mCJlEq1J&payment_intent_data[shipping][tracking_number]=XGOZIrLZf0&payment_method_options[paynow][setup_future_usage]=none&payment_method_options[revolut_pay][setup_future_usage]=off_session&payment_method_options[klarna][setup_future_usage]=none&payment_intent_data[shipping][address][postal_code]=1aAilmcYiq&subscription_data[invoice_settings][issuer][account]=axhiYamJKY&subscription_data[trial_period_days]=1684102049&subscription_data[description]=7mpaD2E0jf&cancel_url=qpmWppPyIv&payment_method_options[card][statement_descriptor_suffix_kana]=ZvJtIONyDK&payment_method_options[pix][expires_after_seconds]=191312234&custom_fields[0][dropdown][options][0][value]=hXN8MppU0k&tax_id_collection[enabled]=true&payment_method_options[sofort][setup_future_usage]=none&payment_method_options[customer_balance][setup_future_usage]=none&payment_method_options[ideal][setup_future_usage]=none&payment_intent_data[description]=yoalRHw9ZG&payment_intent_data[shipping][phone]=CWAbvZM4Kw&expires_at=1756067225&line_items[0][adjustable_quantity][enabled]=false&invoice_creation[invoice_data][description]=MiePp9LfkQ&payment_method_options[card][request_three_d_secure]=any&payment_method_options[customer_balance][bank_transfer][requested_address_types][]=iban&line_items[0][price_data][unit_amount_decimal]=vkJPCvrn9Q&phone_number_collection[enabled]=true&payment_intent_data[shipping][address][line2]=CM9x9Jizzu&subscription_data[billing_cycle_anchor]=1981798554&subscription_data[application_fee_percent]=1.7020678102144877"
		values, err := url.ParseQuery(complexValue)
		require.NoError(b, err)

		for b.Loop() {
			_, errs := parseDeepObjectNodes(values)
			if len(errs) > 0 {
				panic(errs)
			}
		}
	})
}
